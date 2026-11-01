// Package saga books an FX conversion across two ledgers. A conversion touches the sell asset's ledger and the buy asset's
// ledger, and no single transfer can span them, so the saga holds both legs, posts the buy, then posts the sell. Every step
// has a fixed idempotency key and is recorded when it completes, so a crash resumes from the recorded step.
//
// If the sell cannot be posted after the buy was, the buy is reversed. A failure before anything is posted voids the holds.
package saga

import (
	"context"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	ledgerv1 "github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1/ledgerv1connect"
	"github.com/khayaai/forge-mutual/pkg/outbox"
	"github.com/khayaai/forge-mutual/services/bank/fx-service/internal/store"
)

// Transfer codes (Appendix B).
const (
	codeFXLeg    = 20 // FX_LEG
	codeFXSpread = 21 // FX_SPREAD
	codeReversal = 90 // REVERSAL
)

// Ledger account codes.
const (
	codeFXPool = 2003
	codeFees   = 2002
)

// Topic is the deal-completed event.
const Topic = "forge.bank.fx.booked.v1"

// Saga runs deal steps.
type Saga struct {
	Pool   *pgxpool.Pool
	Ledger ledgerv1connect.LedgerServiceClient
	Entity string
	Now    func() time.Time
	// Accounts returns the FX pool and the fee account for an asset.
	Accounts func(ctx context.Context, asset string) (pool, fees string, err error)
}

// ErrBusy means another worker is driving this deal. The caller leaves it to that worker.
var ErrBusy = errors.New("deal is being advanced by another worker")

// Run advances a deal as far as it can. It returns the state it reached. An error means the step is uncertain and the deal
// is still in a non-final state: calling Run again continues from the same place. Only one worker runs a deal at a time.
func (s *Saga) Run(ctx context.Context, dealID string) (string, error) {
	unlock, ok, err := s.lock(ctx, dealID)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", ErrBusy
	}
	defer unlock()

	d, err := store.GetDealByID(ctx, s.Pool, dealID)
	if err != nil {
		return "", err
	}
	sellPool, sellFees, err := s.Accounts(ctx, d.SellAsset)
	if err != nil {
		return d.State, err
	}
	buyPool, _, err := s.Accounts(ctx, d.BuyAsset)
	if err != nil {
		return d.State, err
	}
	midPart := d.SellAmount - d.SpreadMinor

	for !store.IsFinal(d.State) {
		switch d.State {
		case store.StateBooked:
			legs := []*ledgerv1.TransferLeg{{DebitAccountId: d.SellAccount, CreditAccountId: sellPool,
				Amount: money(d.SellAsset, midPart), Code: codeFXLeg}}
			if d.SpreadMinor > 0 {
				legs = append(legs, &ledgerv1.TransferLeg{DebitAccountId: d.SellAccount, CreditAccountId: sellFees,
					Amount: money(d.SellAsset, d.SpreadMinor), Code: codeFXSpread})
			}
			ids, status, err := s.hold(ctx, dealID+":sell:hold", legs)
			if err != nil {
				return d.State, err
			}
			if status != ledgerv1.Status_STATUS_OK && status != ledgerv1.Status_STATUS_REPLAYED {
				return s.finish(ctx, d, store.StateVoided, "sell leg refused by the ledger", nil)
			}
			if err := store.Advance(ctx, s.Pool, dealID, store.StateSellPending, "", ids, nil, nil); err != nil {
				return d.State, err
			}
			d.State, d.SellHolds = store.StateSellPending, ids

		case store.StateSellPending:
			legs := []*ledgerv1.TransferLeg{{DebitAccountId: buyPool, CreditAccountId: d.BuyAccount,
				Amount: money(d.BuyAsset, d.BuyAmount), Code: codeFXLeg}}
			ids, status, err := s.hold(ctx, dealID+":buy:hold", legs)
			if err != nil {
				return d.State, err
			}
			if status != ledgerv1.Status_STATUS_OK && status != ledgerv1.Status_STATUS_REPLAYED {
				// The buy cannot be funded, so release the sell hold. Nothing has moved.
				if err := s.void(ctx, dealID+":sell:void", d.SellHolds); err != nil {
					return d.State, err
				}
				return s.finish(ctx, d, store.StateVoided, "buy leg refused by the ledger", nil)
			}
			if err := store.Advance(ctx, s.Pool, dealID, store.StateBuyPending, "", nil, ids, nil); err != nil {
				return d.State, err
			}
			d.State, d.BuyHolds = store.StateBuyPending, ids

		case store.StateBuyPending:
			if err := s.post(ctx, dealID+":buy:post", d.BuyHolds); err != nil {
				return d.State, err
			}
			if err := store.Advance(ctx, s.Pool, dealID, store.StateBuyPosted, "", nil, nil, nil); err != nil {
				return d.State, err
			}
			d.State = store.StateBuyPosted

		case store.StateBuyPosted:
			err := s.post(ctx, dealID+":sell:post", d.SellHolds)
			if err == nil {
				ev := bookedEvent(dealID, d, s.entity(), s.now())
				return s.finish(ctx, d, store.StateCompleted, "", &ev)
			}
			if !definitive(err) {
				return d.State, err // uncertain: the sell may have posted. Try again; never reverse on a guess.
			}
			// The sell was refused: undo the buy, so the customer does not keep USD for a conversion that did not happen.
			if err := s.reverse(ctx, dealID+":buy:reverse", d.BuyAccount, buyPool, d.BuyAsset, d.BuyAmount); err != nil {
				return d.State, err
			}
			return s.finish(ctx, d, store.StateReversed, "sell leg refused after the buy posted; the buy was reversed", nil)

		default:
			return d.State, fmt.Errorf("saga: unknown state %q", d.State)
		}
	}
	return d.State, nil
}

// lock takes a session advisory lock on the deal. The lock lives on its own connection, outside the pool, for the whole
// run, so a booking and the resume loop, or two replicas, cannot drive the same deal at once. Running out of pool
// connections cannot block it, and closing the connection releases the lock, so a run cannot leave it behind.
func (s *Saga) lock(ctx context.Context, dealID string) (func(), bool, error) {
	conn, err := pgx.ConnectConfig(ctx, s.Pool.Config().ConnConfig.Copy())
	if err != nil {
		return nil, false, err
	}
	key := "fx-deal:" + dealID
	var ok bool
	if err := conn.QueryRow(ctx, `select pg_try_advisory_lock(hashtextextended($1, 0))`, key).Scan(&ok); err != nil {
		_ = conn.Close(ctx)
		return nil, false, err
	}
	if !ok {
		_ = conn.Close(ctx)
		return nil, false, nil
	}
	return func() {
		uctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = conn.Close(uctx)
	}, true, nil
}

func (s *Saga) finish(ctx context.Context, d store.Deal, state, reason string, ev *outbox.Event) (string, error) {
	if err := store.Advance(ctx, s.Pool, d.DealID, state, reason, nil, nil, ev); err != nil {
		return d.State, err
	}
	return state, nil
}

// hold creates pending transfers under a key and returns their ids. A rejection is a status, not an error.
func (s *Saga) hold(ctx context.Context, key string, legs []*ledgerv1.TransferLeg) ([]string, ledgerv1.Status, error) {
	res, err := s.Ledger.CreateTransfers(ctx, connect.NewRequest(&ledgerv1.CreateTransfersRequest{
		IdempotencyKey: key, BusinessTxnId: key, EntityCode: s.entity(), Legs: legs, Pending: true,
		PendingTimeoutSeconds: 300, Actor: "fx-service", Reason: "fx conversion hold",
	}))
	if err != nil {
		return nil, 0, err
	}
	return res.Msg.TransferIds, res.Msg.Status, nil
}

func (s *Saga) post(ctx context.Context, key string, holds []string) error {
	res, err := s.Ledger.PostPending(ctx, connect.NewRequest(&ledgerv1.PostPendingRequest{
		IdempotencyKey: key, BusinessTxnId: key, EntityCode: s.entity(), PendingTransferIds: holds,
		Actor: "fx-service", Reason: "fx conversion",
	}))
	if err != nil {
		return err
	}
	if res.Msg.Status != ledgerv1.Status_STATUS_OK && res.Msg.Status != ledgerv1.Status_STATUS_REPLAYED {
		return &refused{status: res.Msg.Status.String()}
	}
	return nil
}

func (s *Saga) void(ctx context.Context, key string, holds []string) error {
	res, err := s.Ledger.VoidPending(ctx, connect.NewRequest(&ledgerv1.VoidPendingRequest{
		IdempotencyKey: key, BusinessTxnId: key, EntityCode: s.entity(), PendingTransferIds: holds,
		Actor: "fx-service", Reason: "fx conversion released",
	}))
	if err != nil {
		return err
	}
	if res.Msg.Status != ledgerv1.Status_STATUS_OK && res.Msg.Status != ledgerv1.Status_STATUS_REPLAYED {
		return &refused{status: res.Msg.Status.String()}
	}
	return nil
}

func (s *Saga) reverse(ctx context.Context, key, customer, pool, asset string, amount int64) error {
	res, err := s.Ledger.CreateTransfers(ctx, connect.NewRequest(&ledgerv1.CreateTransfersRequest{
		IdempotencyKey: key, BusinessTxnId: key, EntityCode: s.entity(),
		Legs:  []*ledgerv1.TransferLeg{{DebitAccountId: customer, CreditAccountId: pool, Amount: money(asset, amount), Code: codeReversal}},
		Actor: "fx-service", Reason: "fx conversion reversed",
	}))
	if err != nil {
		return err
	}
	if res.Msg.Status != ledgerv1.Status_STATUS_OK && res.Msg.Status != ledgerv1.Status_STATUS_REPLAYED {
		return &refused{status: res.Msg.Status.String()}
	}
	return nil
}

// refused is a definitive answer from the ledger: nothing was written.
type refused struct{ status string }

func (r *refused) Error() string { return "ledger refused: " + r.status }

// definitive reports whether an error means the ledger certainly did not write. Only an explicit refusal, or a client
// error from the ledger, qualifies. A timeout or an unavailable service does not: the write may have happened.
func definitive(err error) bool {
	var r *refused
	if errors.As(err, &r) {
		return true
	}
	switch connect.CodeOf(err) {
	case connect.CodeInvalidArgument, connect.CodeFailedPrecondition, connect.CodeNotFound:
		return true
	}
	return false
}

func money(asset string, amount int64) *ledgerv1.Money {
	return &ledgerv1.Money{Asset: asset, AmountMinor: fmt.Sprint(amount)}
}

func (s *Saga) entity() string {
	if s.Entity != "" {
		return s.Entity
	}
	return "BANK"
}

func (s *Saga) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}
