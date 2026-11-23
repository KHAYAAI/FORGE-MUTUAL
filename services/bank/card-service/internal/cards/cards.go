// Package cards issues virtual debit cards on a customer's own account and decides, in real time, whether each purchase
// is allowed. A purchase is approved only if the card, the customer and the amount pass every rule and the customer's
// account can cover it: the money is held on the ledger at that moment, posted at clearing, released on reversal, and
// returned on refund. Card numbers never reach Forge.
package cards

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	ledgerv1 "github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1/ledgerv1connect"
	"github.com/khayaai/forge-mutual/pkg/licence"
	"github.com/khayaai/forge-mutual/pkg/systemaccounts"
)

//go:embed schema.sql
var schema string

// Open applies the schema.
func Open(ctx context.Context, pool *pgxpool.Pool) error {
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `select pg_advisory_xact_lock(7761)`); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, schema)
		return err
	})
}

// Ownership reports the accounts a party owns and their assets.
type Ownership interface {
	Accounts(ctx context.Context, party string) (map[string]string, error)
}

// Parties reports whether a party may act. Only ACTIVE parties may spend: a frozen party (a sanctions match) cannot.
type Parties interface {
	Active(ctx context.Context, party string) (bool, error)
}

const (
	codeHold   = 50 // CARD_AUTH_HOLD
	codeSettle = 51 // CARD_SETTLE
	// HoldSeconds is how long an authorisation holds funds: the ledger's maximum pending time, seven days.
	HoldSeconds = 7 * 24 * 60 * 60
)

// Decline reasons. They are for the cardholder and the processor: never a financial-crime reason (no tipping off).
const (
	ReasonUnknownCard   = "CARD_UNKNOWN"
	ReasonNotActive     = "CARD_NOT_ACTIVE"
	ReasonCurrency      = "CURRENCY_NOT_SUPPORTED"
	ReasonMerchant      = "MERCHANT_CATEGORY_BLOCKED"
	ReasonPerTxn        = "LIMIT_PER_TRANSACTION"
	ReasonDaily         = "LIMIT_DAILY"
	ReasonFunds         = "INSUFFICIENT_FUNDS"
	ReasonUnavailable   = "TEMPORARILY_UNAVAILABLE"
	ReasonNotAvailable  = "NOT_AVAILABLE"
	ReasonAccountStatus = "ACCOUNT_RESTRICTED"
)

var sast = time.FixedZone("SAST", 2*60*60) // South Africa has no daylight saving

// Service is the card service.
type Service struct {
	Pool      *pgxpool.Pool
	Ledger    ledgerv1connect.LedgerServiceClient
	Owners    Ownership
	Parties   Parties
	Processor Processor
	Licence   licence.Gate
	Entity    string
	Log       *slog.Logger
	Now       func() time.Time

	// Limits, in rand cents. PROPOSED placeholders (docs/THRESHOLDS.md, thresholds 29 and 30): a card starts with the
	// defaults, its owner may change them up to the ceilings.
	DefaultPerTxn, DefaultDaily int64
	MaxPerTxn, MaxDaily         int64
	MaxCardsPerParty            int
	// BlockedMCC lists merchant category codes that are always declined (PROPOSED: gambling).
	BlockedMCC map[string]bool

	Settlement string // CARD_SCHEME_SETTLEMENT, set by EnsureSystemAccounts
}

// EnsureSystemAccounts creates (or finds) the scheme settlement account.
func (s *Service) EnsureSystemAccounts(ctx context.Context) error {
	var err error
	s.Settlement, err = systemaccounts.Ensure(ctx, s.Ledger, s.Entity, "ZAR", 3030, "system:card-scheme-settlement:ZAR:v1")
	return err
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

// Decision is the answer to an authorisation request.
type Decision struct {
	Approved bool   `json:"approved"`
	Reason   string `json:"reason,omitempty"`
	AuthCode string `json:"auth_code,omitempty"`
}

func authCode(id string) string {
	h := sha256.Sum256([]byte("card-auth-code:" + id))
	return strings.ToUpper(hex.EncodeToString(h[:3]))
}

func approved(id string) Decision { return Decision{Approved: true, AuthCode: authCode(id)} }
func declined(reason string) Decision {
	return Decision{Reason: reason}
}

type cardRow struct {
	ID, Party, Account, Status string
	PerTxn, Daily              int64
}

func (s *Service) cardByToken(ctx context.Context, token string) (cardRow, error) {
	var c cardRow
	err := s.Pool.QueryRow(ctx, `select card_id, party_id, account_id, status, per_txn_limit, daily_limit from card where token = $1`, token).
		Scan(&c.ID, &c.Party, &c.Account, &c.Status, &c.PerTxn, &c.Daily)
	return c, err
}

func (s *Service) leg(from, to string, amount int64, code uint32) *ledgerv1.TransferLeg {
	return &ledgerv1.TransferLeg{DebitAccountId: from, CreditAccountId: to, Code: code,
		Amount: &ledgerv1.Money{Asset: "ZAR", AmountMinor: fmt.Sprintf("%d", amount)}}
}

func (s *Service) transfer(ctx context.Context, key, reason string, pending bool, legs ...*ledgerv1.TransferLeg) ([]string, ledgerv1.Status, []string, error) {
	req := &ledgerv1.CreateTransfersRequest{IdempotencyKey: key, BusinessTxnId: key, EntityCode: s.Entity, Legs: legs,
		Pending: pending, Actor: "svc-cards", Reason: reason}
	if pending {
		req.PendingTimeoutSeconds = HoldSeconds
	}
	res, err := s.Ledger.CreateTransfers(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, 0, nil, err
	}
	return res.Msg.TransferIds, res.Msg.Status, res.Msg.RejectionCodes, nil
}

func (s *Service) settle(ctx context.Context, key string, ids []string, post bool) error {
	if post {
		res, err := s.Ledger.PostPending(ctx, connect.NewRequest(&ledgerv1.PostPendingRequest{IdempotencyKey: key, BusinessTxnId: key,
			EntityCode: s.Entity, PendingTransferIds: ids, Actor: "svc-cards", Reason: "card clearing"}))
		if err != nil {
			return err
		}
		if res.Msg.Status != ledgerv1.Status_STATUS_OK && res.Msg.Status != ledgerv1.Status_STATUS_REPLAYED {
			return fmt.Errorf("%w: %v", errRefused, res.Msg.RejectionCodes)
		}
		return nil
	}
	res, err := s.Ledger.VoidPending(ctx, connect.NewRequest(&ledgerv1.VoidPendingRequest{IdempotencyKey: key, BusinessTxnId: key,
		EntityCode: s.Entity, PendingTransferIds: ids, Actor: "svc-cards", Reason: "card hold released"}))
	if err != nil {
		return err
	}
	if res.Msg.Status != ledgerv1.Status_STATUS_OK && res.Msg.Status != ledgerv1.Status_STATUS_REPLAYED {
		return fmt.Errorf("%w: %v", errRefused, res.Msg.RejectionCodes)
	}
	return nil
}

// errRefused means the ledger answered and said no, as opposed to being unreachable.
var errRefused = errors.New("cards: the ledger refused")

func spentToday(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, cardID string, now time.Time) (int64, error) {
	start := time.Date(now.In(sast).Year(), now.In(sast).Month(), now.In(sast).Day(), 0, 0, 0, 0, sast)
	var n int64
	err := q.QueryRow(ctx, `select coalesce(sum(case when status in ('PENDING','HELD') then amount_minor
		when status in ('CAPTURED','REFUNDED') then captured_minor else 0 end), 0)
		from card_auth where card_id = $1 and created_at >= $2`, cardID, start).Scan(&n)
	return n, err
}

func (s *Service) record(ctx context.Context, ev Event, cardID, status, reason string) error {
	now := s.now()
	_, err := s.Pool.Exec(ctx, `insert into card_auth (auth_id, card_id, amount_minor, currency, merchant, mcc, country, status, decline_reason, created_at, updated_at)
		values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$10) on conflict (auth_id) do nothing`,
		ev.ID, cardID, ev.AmountMinor, ev.Currency, ev.Merchant, ev.MCC, ev.Country, status, reason, now)
	return err
}

// Authorise decides one purchase. The processor's authorisation id makes it idempotent: asking again returns the same
// answer and never holds funds twice. Every refusal is final for that id; a decline never leaves money held.
func (s *Service) Authorise(ctx context.Context, ev Event) (Decision, error) {
	if ev.AmountMinor <= 0 {
		return declined(ReasonNotAvailable), nil
	}
	// A replay returns what was decided.
	var status, reason string
	err := s.Pool.QueryRow(ctx, `select status, decline_reason from card_auth where auth_id = $1`, ev.ID).Scan(&status, &reason)
	switch {
	case err == nil && status == "DECLINED":
		return declined(reason), nil
	case err == nil && status == "PENDING":
		return s.hold(ctx, ev.ID) // an earlier attempt did not finish
	case err == nil:
		return approved(ev.ID), nil
	case !errors.Is(err, pgx.ErrNoRows):
		return declined(ReasonUnavailable), err
	}
	if err := s.Licence.Allow(licence.Cards); err != nil {
		return declined(ReasonNotAvailable), nil
	}
	card, err := s.cardByToken(ctx, ev.Token)
	if errors.Is(err, pgx.ErrNoRows) {
		return declined(ReasonUnknownCard), nil // nothing to record it against
	}
	if err != nil {
		return declined(ReasonUnavailable), err
	}
	no := func(why string) (Decision, error) {
		if err := s.record(ctx, ev, card.ID, "DECLINED", why); err != nil {
			return declined(ReasonUnavailable), err
		}
		return declined(why), nil
	}
	switch {
	case card.Status != "ACTIVE":
		return no(ReasonNotActive)
	case ev.Currency != "ZAR":
		return no(ReasonCurrency)
	case s.BlockedMCC[ev.MCC]:
		return no(ReasonMerchant)
	case ev.AmountMinor > card.PerTxn:
		return no(ReasonPerTxn)
	}
	if s.Parties != nil {
		ok, err := s.Parties.Active(ctx, card.Party)
		if err != nil {
			return declined(ReasonUnavailable), err
		}
		if !ok {
			return no(ReasonAccountStatus)
		}
	}
	// The daily limit and the new row are decided under one lock per card, so two purchases at the same moment cannot
	// both pass a limit only one fits under.
	var over bool
	err = pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `select pg_advisory_xact_lock(hashtext('card:' || $1))`, card.ID); err != nil {
			return err
		}
		spent, err := spentToday(ctx, tx, card.ID, s.now())
		if err != nil {
			return err
		}
		if spent+ev.AmountMinor > card.Daily {
			over = true
			return nil
		}
		now := s.now()
		_, err = tx.Exec(ctx, `insert into card_auth (auth_id, card_id, amount_minor, currency, merchant, mcc, country, status, created_at, updated_at)
			values ($1,$2,$3,$4,$5,$6,$7,'PENDING',$8,$8) on conflict (auth_id) do nothing`,
			ev.ID, card.ID, ev.AmountMinor, ev.Currency, ev.Merchant, ev.MCC, ev.Country, now)
		return err
	})
	if err != nil {
		return declined(ReasonUnavailable), err
	}
	if over {
		return no(ReasonDaily)
	}
	return s.hold(ctx, ev.ID)
}

// hold puts the funds on hold for a PENDING authorisation and records the outcome. It is safe to repeat: the ledger key
// is the authorisation id.
func (s *Service) hold(ctx context.Context, authID string) (Decision, error) {
	var account string
	var amount int64
	var status string
	err := s.Pool.QueryRow(ctx, `select c.account_id, a.amount_minor, a.status from card_auth a join card c using (card_id) where a.auth_id = $1`, authID).
		Scan(&account, &amount, &status)
	if err != nil {
		return declined(ReasonUnavailable), err
	}
	if status != "PENDING" {
		if status == "DECLINED" {
			return declined(ReasonUnavailable), nil
		}
		return approved(authID), nil
	}
	ids, st, codes, err := s.transfer(ctx, "card-auth:"+authID, "card purchase hold", true, s.leg(account, s.Settlement, amount, codeHold))
	if err != nil {
		// The ledger could not be asked. The processor is told no; Recover finishes the row.
		return declined(ReasonUnavailable), nil
	}
	if st != ledgerv1.Status_STATUS_OK && st != ledgerv1.Status_STATUS_REPLAYED {
		why := ReasonNotAvailable
		for _, c := range codes {
			if strings.Contains(c, "insufficient") {
				why = ReasonFunds
			}
		}
		_, err := s.Pool.Exec(ctx, `update card_auth set status='DECLINED', decline_reason=$2, updated_at=$3 where auth_id=$1 and status='PENDING'`, authID, why, s.now())
		return declined(why), err
	}
	if len(ids) == 0 {
		return declined(ReasonUnavailable), errors.New("cards: the ledger returned no hold")
	}
	if _, err := s.Pool.Exec(ctx, `update card_auth set status='HELD', hold_transfer_id=$2, updated_at=$3 where auth_id=$1 and status='PENDING'`, authID, ids[0], s.now()); err != nil {
		return declined(ReasonUnavailable), err
	}
	return approved(authID), nil
}

type authRow struct {
	ID, Card, Account, Status, Hold, Clearing string
	Amount, Captured, Refunded                int64
}

func (s *Service) auth(ctx context.Context, id string) (authRow, error) {
	var a authRow
	err := s.Pool.QueryRow(ctx, `select a.auth_id, a.card_id, c.account_id, a.status, a.hold_transfer_id, a.clearing_id, a.amount_minor, a.captured_minor, a.refunded_minor
		from card_auth a join card c using (card_id) where a.auth_id = $1`, id).
		Scan(&a.ID, &a.Card, &a.Account, &a.Status, &a.Hold, &a.Clearing, &a.Amount, &a.Captured, &a.Refunded)
	return a, err
}

func (s *Service) except(ctx context.Context, id, why string) error {
	_, err := s.Pool.Exec(ctx, `update card_auth set status='EXCEPTION', exception=$2, updated_at=$3 where auth_id=$1`, id, why, s.now())
	return err
}

// Clear records that the merchant has been paid. Equal to the hold: the hold is posted. Less: the hold is released and the
// smaller amount is posted. More, or for an authorisation that was declined or reversed: nothing is posted and an
// operator must look at it (EXCEPTION); an expired hold is posted directly if the funds are there.
func (s *Service) Clear(ctx context.Context, ev Event) error {
	a, err := s.auth(ctx, ev.AuthID)
	if err != nil {
		return err
	}
	if a.Clearing == ev.ID && a.Status == "CAPTURED" {
		return nil // the same clearing again
	}
	amt := ev.AmountMinor
	if amt <= 0 {
		return s.except(ctx, a.ID, "CLEARING_AMOUNT_INVALID")
	}
	switch a.Status {
	case "HELD":
		switch {
		case amt == a.Amount:
			if err := s.settle(ctx, "card-post:"+a.ID, []string{a.Hold}, true); err != nil {
				if errors.Is(err, errRefused) {
					return s.directPost(ctx, a, amt, "card-capture:"+a.ID, ev.ID) // the hold is gone: try the account
				}
				return err
			}
		case amt < a.Amount:
			if err := s.settle(ctx, "card-void:"+a.ID, []string{a.Hold}, false); err != nil && !errors.Is(err, errRefused) {
				return err
			}
			return s.directPost(ctx, a, amt, "card-capture:"+a.ID, ev.ID)
		default:
			return s.except(ctx, a.ID, "OVER_CAPTURE")
		}
	case "EXPIRED":
		return s.directPost(ctx, a, amt, "card-capture:"+a.ID, ev.ID)
	case "CAPTURED", "REFUNDED":
		return s.except(ctx, a.ID, "SECOND_CLEARING")
	default: // PENDING, DECLINED, REVERSED, EXCEPTION
		return s.except(ctx, a.ID, "CLEARING_FOR_"+a.Status)
	}
	_, err = s.Pool.Exec(ctx, `update card_auth set status='CAPTURED', captured_minor=$2, clearing_id=$3, updated_at=$4 where auth_id=$1`, a.ID, amt, ev.ID, s.now())
	return err
}

// directPost posts a purchase straight against the account, with no hold. If the account cannot cover it, nothing moves
// and an operator must look at it.
func (s *Service) directPost(ctx context.Context, a authRow, amt int64, key, clearingID string) error {
	_, st, _, err := s.transfer(ctx, key, "card purchase", false, s.leg(a.Account, s.Settlement, amt, codeSettle))
	if err != nil {
		return err
	}
	if st != ledgerv1.Status_STATUS_OK && st != ledgerv1.Status_STATUS_REPLAYED {
		return s.except(ctx, a.ID, "CAPTURE_FAILED")
	}
	_, err = s.Pool.Exec(ctx, `update card_auth set status='CAPTURED', captured_minor=$2, clearing_id=$3, updated_at=$4 where auth_id=$1`, a.ID, amt, clearingID, s.now())
	return err
}

// Reverse releases an authorisation that will not be cleared.
func (s *Service) Reverse(ctx context.Context, ev Event) error {
	a, err := s.auth(ctx, ev.AuthID)
	if err != nil {
		return err
	}
	if a.Status != "HELD" {
		return nil // already reversed, declined, expired or cleared: nothing is held
	}
	if err := s.settle(ctx, "card-void:"+a.ID, []string{a.Hold}, false); err != nil && !errors.Is(err, errRefused) {
		return err
	}
	_, err = s.Pool.Exec(ctx, `update card_auth set status='REVERSED', updated_at=$2 where auth_id=$1 and status='HELD'`, a.ID, s.now())
	return err
}

// Refund returns money for a cleared purchase, up to what was captured less what was already refunded.
func (s *Service) Refund(ctx context.Context, ev Event) error {
	a, err := s.auth(ctx, ev.AuthID)
	if err != nil {
		return err
	}
	if a.Status != "CAPTURED" && a.Status != "REFUNDED" {
		return s.except(ctx, a.ID, "REFUND_FOR_"+a.Status)
	}
	if ev.AmountMinor <= 0 || ev.AmountMinor > a.Captured-a.Refunded {
		var done bool
		_ = s.Pool.QueryRow(ctx, `select exists(select 1 from card_refund where refund_id = $1)`, ev.ID).Scan(&done)
		if done {
			return nil // this refund was already applied
		}
		return s.except(ctx, a.ID, "REFUND_EXCEEDS_PURCHASE")
	}
	_, st, _, err := s.transfer(ctx, "card-refund:"+ev.ID, "card refund", false, s.leg(s.Settlement, a.Account, ev.AmountMinor, codeSettle))
	if err != nil {
		return err
	}
	if st != ledgerv1.Status_STATUS_OK && st != ledgerv1.Status_STATUS_REPLAYED {
		return s.except(ctx, a.ID, "REFUND_FAILED")
	}
	return pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `insert into card_refund (refund_id, auth_id, amount_minor, at) values ($1,$2,$3,$4) on conflict do nothing`, ev.ID, a.ID, ev.AmountMinor, s.now())
		if err != nil || tag.RowsAffected() == 0 {
			return err // a replay: already counted
		}
		_, err = tx.Exec(ctx, `update card_auth set refunded_minor = refunded_minor + $2,
			status = case when refunded_minor + $2 >= captured_minor then 'REFUNDED' else status end, updated_at = $3 where auth_id = $1`, a.ID, ev.AmountMinor, s.now())
		return err
	})
}

// Recover finishes work an interruption left half done. An authorisation still PENDING after two minutes was answered
// "declined" (the processor timed out or the ledger was unreachable), so any hold it made is released and the row is
// closed as declined. Holds older than the ledger's timeout are marked EXPIRED: the ledger has already released them.
func (s *Service) Recover(ctx context.Context) (int, error) {
	n := 0
	rows, err := s.Pool.Query(ctx, `select auth_id from card_auth where status='PENDING' and created_at < $1`, s.now().Add(-2*time.Minute))
	if err != nil {
		return 0, err
	}
	var pend []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		pend = append(pend, id)
	}
	rows.Close()
	for _, id := range pend {
		d, err := s.hold(ctx, id) // idempotent: finds the hold if one was made, makes it if not
		if err != nil {
			return n, err
		}
		if !d.Approved {
			continue // already closed as declined, or the ledger is still away: try again next time
		}
		a, err := s.auth(ctx, id)
		if err != nil {
			return n, err
		}
		// The processor was told no. Release the hold and close the row.
		if err := s.settle(ctx, "card-void-late:"+id, []string{a.Hold}, false); err != nil && !errors.Is(err, errRefused) {
			return n, err
		}
		if _, err := s.Pool.Exec(ctx, `update card_auth set status='DECLINED', decline_reason=$2, updated_at=$3 where auth_id=$1 and status='HELD'`, id, ReasonUnavailable, s.now()); err != nil {
			return n, err
		}
		n++
	}
	tag, err := s.Pool.Exec(ctx, `update card_auth set status='EXPIRED', updated_at=$2 where status='HELD' and created_at < $1`,
		s.now().Add(-time.Duration(HoldSeconds)*time.Second), s.now())
	if err != nil {
		return n, err
	}
	return n + int(tag.RowsAffected()), nil
}

// Webhook receives the processor's deliveries. It authenticates itself by signature, not by a caller certificate.
// An authorisation is answered with the decision; the rest with 200 once applied, so the processor retries on failure.
func (s *Service) Webhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 64<<10))
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	ev, err := s.Processor.VerifyWebhook(body, r.Header)
	if err != nil {
		http.Error(w, "unauthorised", http.StatusUnauthorized)
		return
	}
	ctx := r.Context()
	switch ev.Type {
	case EventAuthorisation:
		d, err := s.Authorise(ctx, ev)
		if err != nil {
			s.Log.Error("authorisation failed", "auth", ev.ID, "err", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"approved":%t,"reason":%q,"auth_code":%q}`, d.Approved, d.Reason, d.AuthCode)
		return
	case EventClearing:
		err = s.Clear(ctx, ev)
	case EventReversal:
		err = s.Reverse(ctx, ev)
	case EventRefund:
		err = s.Refund(ctx, ev)
	}
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "unknown authorisation", http.StatusNotFound)
			return
		}
		s.Log.Error("card event failed; the processor will retry", "type", ev.Type, "id", ev.ID, "err", err)
		http.Error(w, "try again", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
}
