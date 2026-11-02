// Package inbound books credits the bank reports. Each credit is posted once, under an idempotency key derived from
// the bank's movement, so a retry after a crash returns the original posting and never books twice.
package inbound

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"strconv"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	eventsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/events/v1"
	ledgerv1 "github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1/ledgerv1connect"
	"github.com/khayaai/forge-mutual/pkg/banklayer"
	"github.com/khayaai/forge-mutual/pkg/events"
	"github.com/khayaai/forge-mutual/pkg/outbox"
	"github.com/khayaai/forge-mutual/pkg/systemaccounts"
	"github.com/khayaai/forge-mutual/pkg/uuid"
)

// TopicCreditBooked carries a credit booked to a customer's account (eventsv1.InboundCreditBooked).
const TopicCreditBooked = "forge.bank.inbound.credit.booked.v1"

//go:embed schema.sql
var schema string

const (
	codeDepositIn = 10   // DEPOSIT_IN (Appendix B)
	codeSuspense  = 2005 // SUSPENSE
	codeCustomer  = 1001 // CUSTOMER_DEPOSIT
	namespace     = "3b1f0c2e-7d4a-5f6e-9b8c-2a1d0e9f8b7c"
)

// poolCode is the bank-side mirror that a credit debits: the sponsor pool for ZAR, the partner mirror for USD.
var poolCode = map[string]uint16{"ZAR": 3003, "USD": 5001}

// Statuses.
const (
	StatusPending  = "PENDING"
	StatusBooked   = "BOOKED"
	StatusSuspense = "SUSPENSE"
)

// Worker books inbound credits for one entity.
type Worker struct {
	Pool    *pgxpool.Pool
	Ledger  ledgerv1connect.LedgerServiceClient
	Bank    banklayer.BankLayer
	Entity  string
	Now     func() time.Time
	systems map[string]systemAccounts
}

type systemAccounts struct {
	pool     string
	suspense string
}

// Open applies the schema. It is idempotent.
func Open(ctx context.Context, pool *pgxpool.Pool) error {
	if err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `select pg_advisory_xact_lock(7741)`); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, schema)
		return err
	}); err != nil {
		return err
	}
	// Credits to customers are published through the outbox, which lives in the same database.
	if err := outbox.Migrate(ctx, pool); err != nil {
		return fmt.Errorf("inbound: outbox: %w", err)
	}
	return nil
}

// RegisterReference binds a bank reference to a customer account. Re-registering the same binding is a no-op;
// binding a reference to a different account is refused, because a reference that moves would misdirect money.
func (w *Worker) RegisterReference(ctx context.Context, reference, asset, accountID, partyID string) error {
	if reference == "" || asset == "" || accountID == "" || partyID == "" {
		return errors.New("inbound: reference, asset, account and party are required")
	}
	tag, err := w.Pool.Exec(ctx, `
		insert into inbound_reference (reference, asset, account_id, party_id)
		values ($1, $2, $3, $4) on conflict (reference) do nothing`, reference, asset, accountID, partyID)
	if err != nil {
		return fmt.Errorf("inbound: register: %w", err)
	}
	if tag.RowsAffected() == 1 {
		return nil
	}
	var a, p string
	if err := w.Pool.QueryRow(ctx, `select account_id, party_id from inbound_reference where reference = $1`, reference).Scan(&a, &p); err != nil {
		return fmt.Errorf("inbound: register: %w", err)
	}
	if a != accountID || p != partyID {
		return fmt.Errorf("inbound: reference %s is already bound to another account", reference)
	}
	return nil
}

// InFlight returns the total of credits the bank has reported that the books do not yet hold. Reconciliation reads
// this as the inbound in-flight term.
func (w *Worker) InFlight(ctx context.Context, asset string) (int64, error) {
	var total int64
	err := w.Pool.QueryRow(ctx, `select coalesce(sum(amount_minor), 0) from inbound_credit where asset = $1 and status = $2`,
		asset, StatusPending).Scan(&total)
	return total, err
}

// Ingest reads the bank's inbound credits for asset since the lookback, records new ones, and books every pending
// credit. It returns how many credits it booked.
func (w *Worker) Ingest(ctx context.Context, asset string, lookback time.Duration) (int, error) {
	now := w.Now
	if now == nil {
		now = time.Now
	}
	sys, err := w.systemAccountsFor(ctx, asset)
	if err != nil {
		return 0, err
	}
	moves, err := w.inboundFor(ctx, asset, now().Add(-lookback))
	if err != nil {
		return 0, err
	}
	for _, m := range moves {
		if m.Direction != "IN" {
			continue
		}
		if _, err := w.Pool.Exec(ctx, `
			insert into inbound_credit (asset, movement_id, amount_minor, reference, status)
			values ($1, $2, $3, $4, 'PENDING') on conflict (asset, movement_id) do nothing`,
			asset, m.ID, m.AmountMinor, m.Reference); err != nil {
			return 0, fmt.Errorf("inbound: record: %w", err)
		}
	}

	rows, err := w.Pool.Query(ctx, `
		select movement_id, amount_minor, reference from inbound_credit
		where asset = $1 and status = 'PENDING' order by seen_at, movement_id limit 500`, asset)
	if err != nil {
		return 0, fmt.Errorf("inbound: pending: %w", err)
	}
	type pending struct {
		id, ref string
		amount  int64
	}
	var todo []pending
	for rows.Next() {
		var p pending
		if err := rows.Scan(&p.id, &p.amount, &p.ref); err != nil {
			rows.Close()
			return 0, err
		}
		todo = append(todo, p)
	}
	rows.Close()

	booked := 0
	for _, p := range todo {
		target, status, err := w.resolve(ctx, asset, p.ref, sys)
		if err != nil {
			return booked, err
		}
		if err := w.post(ctx, asset, p.id, p.amount, p.ref, target, sys); err != nil {
			return booked, err
		}
		var partyID string
		if status == StatusBooked {
			if err := w.Pool.QueryRow(ctx, `select party_id from inbound_reference where reference = $1`, p.ref).Scan(&partyID); err != nil {
				return booked, fmt.Errorf("inbound: party: %w", err)
			}
		}
		if err := w.markBooked(ctx, asset, p.id, status, target, partyID, p.amount, now().UTC()); err != nil {
			return booked, err
		}
		booked++
	}
	return booked, nil
}

// markBooked records a posted credit. For a customer's account it also publishes the credit, in the same transaction, so
// the event goes out if and only if the booking is recorded. A credit held in suspense belongs to no party and is not
// published.
func (w *Worker) markBooked(ctx context.Context, asset, movementID, status, target, partyID string, amount int64, at time.Time) error {
	return pgx.BeginFunc(ctx, w.Pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			update inbound_credit set status = $3, booked_to = $4, booked_at = $5
			where asset = $1 and movement_id = $2`, asset, movementID, status, target, at); err != nil {
			return fmt.Errorf("inbound: mark booked: %w", err)
		}
		if status != StatusBooked {
			return nil
		}
		eventID, payload, err := events.Wrap(events.Params{
			Type: TopicCreditBooked, Entity: w.Entity, Producer: "payments-orchestrator", Correlation: movementID,
		}, &eventsv1.InboundCreditBooked{
			MovementId: movementID, PartyId: partyID, AccountId: target, Asset: asset,
			AmountMinor: strconv.FormatInt(amount, 10), BookedAt: at.Format(time.RFC3339),
		})
		if err != nil {
			return err
		}
		return outbox.Append(ctx, tx, outbox.Event{EventID: eventID, Topic: TopicCreditBooked, Key: []byte(partyID), Payload: payload})
	})
}

// resolve finds the account a reference names. Anything unmatched goes to suspense, where it waits for an operator.
func (w *Worker) resolve(ctx context.Context, asset, reference string, sys systemAccounts) (string, string, error) {
	var accountAsset, accountID string
	err := w.Pool.QueryRow(ctx, `select asset, account_id from inbound_reference where reference = $1`, reference).Scan(&accountAsset, &accountID)
	if errors.Is(err, pgx.ErrNoRows) {
		return sys.suspense, StatusSuspense, nil
	}
	if err != nil {
		return "", "", fmt.Errorf("inbound: resolve: %w", err)
	}
	if accountAsset != asset {
		return sys.suspense, StatusSuspense, nil
	}
	return accountID, StatusBooked, nil
}

// post books one credit. The idempotency key is the bank's movement, so a repeat returns the original posting.
func (w *Worker) post(ctx context.Context, asset, movementID string, amount int64, reference, target string, sys systemAccounts) error {
	key := asset + ":" + movementID
	txn, err := uuid.NewV5(namespace, key)
	if err != nil {
		return err
	}
	res, err := w.Ledger.CreateTransfers(ctx, connect.NewRequest(&ledgerv1.CreateTransfersRequest{
		IdempotencyKey: "inbound:" + key,
		BusinessTxnId:  txn,
		EntityCode:     w.Entity,
		Legs: []*ledgerv1.TransferLeg{{
			DebitAccountId:  sys.pool,
			CreditAccountId: target,
			Amount:          &ledgerv1.Money{Asset: asset, AmountMinor: strconv.FormatInt(amount, 10)},
			Code:            codeDepositIn,
		}},
		Actor:  "inbound-credits",
		Reason: "inbound credit " + reference,
	}))
	if err != nil {
		return fmt.Errorf("inbound: post %s: %w", key, err)
	}
	switch res.Msg.Status {
	case ledgerv1.Status_STATUS_OK, ledgerv1.Status_STATUS_REPLAYED:
		return nil
	}
	return fmt.Errorf("inbound: ledger refused %s with status %v", key, res.Msg.Status)
}

// systemAccountsFor returns the pool and suspense accounts for an asset, creating them on first use. The keys are
// fixed, so every start returns the same accounts.
func (w *Worker) systemAccountsFor(ctx context.Context, asset string) (systemAccounts, error) {
	if s, ok := w.systems[asset]; ok {
		return s, nil
	}
	pc, ok := poolCode[asset]
	if !ok {
		return systemAccounts{}, fmt.Errorf("inbound: no pool account for %s", asset)
	}
	pool, err := w.ensure(ctx, asset, pc, "system:pool:"+asset+":v1")
	if err != nil {
		return systemAccounts{}, err
	}
	suspense, err := w.ensure(ctx, asset, codeSuspense, "system:suspense:"+asset+":v1")
	if err != nil {
		return systemAccounts{}, err
	}
	s := systemAccounts{pool: pool, suspense: suspense}
	if w.systems == nil {
		w.systems = map[string]systemAccounts{}
	}
	w.systems[asset] = s
	return s, nil
}

func (w *Worker) ensure(ctx context.Context, asset string, code uint16, key string) (string, error) {
	return systemaccounts.Ensure(ctx, w.Ledger, w.Entity, asset, uint32(code), key)
}

// inboundFor reads credits from the bank that holds the asset: the sponsor for ZAR, the partner for USD.
func (w *Worker) inboundFor(ctx context.Context, asset string, since time.Time) ([]banklayer.Movement, error) {
	if asset == "ZAR" {
		return w.Bank.StatementSource().Inbound(ctx, asset, since)
	}
	for _, p := range w.Bank.ForeignCurrency() {
		if p.Asset() == asset {
			return p.Inbound(ctx, since)
		}
	}
	return nil, fmt.Errorf("inbound: no bank for %s", asset)
}
