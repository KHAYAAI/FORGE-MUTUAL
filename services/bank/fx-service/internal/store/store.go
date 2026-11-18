// Package store holds FX quotes and deals. A quote is locked when it is booked; a deal records each saga step so a crash
// resumes from the step it reached.
package store

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/khayaai/forge-mutual/pkg/outbox"
)

//go:embed schema.sql
var schema string

// Deal states.
const (
	StateBooked      = "BOOKED"       // locked, no ledger write yet
	StateSellPending = "SELL_PENDING" // the sell leg is held
	StateBuyPending  = "BUY_PENDING"  // the buy leg is held as well
	StateBuyPosted   = "BUY_POSTED"   // the buy leg moved the customer's money; the sell leg is still held
	StateCompleted   = "COMPLETED"
	StateVoided      = "VOIDED"   // failed before anything posted; the holds were released
	StateReversed    = "REVERSED" // failed after the buy posted; the buy was reversed
)

// IsFinal reports whether a deal needs no further steps.
func IsFinal(state string) bool {
	return state == StateCompleted || state == StateVoided || state == StateReversed
}

// ErrNotFound is returned for a quote or deal the party does not hold, or that does not exist.
var ErrNotFound = errors.New("store: not found")

// ErrNotBookable is returned for a quote that is expired or already booked.
var ErrNotBookable = errors.New("store: quote is expired or already booked")

// Open applies the schema and the outbox table the deal events are written to. It is idempotent.
func Open(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, schema); err != nil {
		return fmt.Errorf("store: schema: %w", err)
	}
	if err := outbox.Migrate(ctx, pool); err != nil {
		return fmt.Errorf("store: outbox: %w", err)
	}
	return nil
}

// Quote is an issued quote.
type Quote struct {
	QuoteID     string
	PartyID     string
	SellAsset   string
	BuyAsset    string
	SellAmount  int64
	BuyAmount   int64
	Mid         int64
	Rate        int64
	SpreadBps   int64
	SpreadMinor int64
	ExpiresAt   time.Time
	Status      string
}

// SaveQuote stores an issued quote.
func SaveQuote(ctx context.Context, tx pgx.Tx, q Quote) error {
	_, err := tx.Exec(ctx, `
		insert into fx_quote (quote_id, party_id, sell_asset, buy_asset, sell_amount, buy_amount, mid, rate, spread_bps, spread_minor, expires_at, status)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, 'ISSUED')`,
		q.QuoteID, q.PartyID, q.SellAsset, q.BuyAsset, q.SellAmount, q.BuyAmount, q.Mid, q.Rate, q.SpreadBps, q.SpreadMinor, q.ExpiresAt.UTC())
	return err
}

// LockQuote books a quote and creates its deal in one transaction. It refuses a quote that is expired, already booked,
// or not the party's. The rate and amounts are the ones issued: the lock is what makes a later market move irrelevant.
// ErrAllowance is returned when a conversion would take an individual over their annual foreign allowance.
var ErrAllowance = errors.New("store: the conversion is more than is left of the annual foreign allowance")

// Allowance limits how much rand an individual may convert to foreign currency in a calendar year. Nil means no limit
// (a company, or no allowance configured).
type Allowance struct {
	LimitMinor int64
}

// AllowanceUsed is the rand an individual has converted to foreign currency in the calendar year of now, counting
// every deal that was not voided or reversed: a conversion that failed gives its allowance back.
func AllowanceUsed(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, partyID string, now time.Time) (int64, error) {
	start := time.Date(now.UTC().Year(), 1, 1, 0, 0, 0, 0, time.UTC)
	var used int64
	err := q.QueryRow(ctx, `select coalesce(sum(sell_amount), 0) from fx_deal where party_id = $1 and sell_asset = 'ZAR'
		and state not in ('VOIDED', 'REVERSED') and created_at >= $2 and created_at < $3`, partyID, start, start.AddDate(1, 0, 0)).Scan(&used)
	return used, err
}

func LockQuote(ctx context.Context, pool *pgxpool.Pool, partyID, quoteID, dealID, sellAcc, buyAcc string, now time.Time, allowance *Allowance) (Quote, error) {
	var q Quote
	err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			select quote_id, party_id, sell_asset, buy_asset, sell_amount, buy_amount, mid, rate, spread_bps, spread_minor, expires_at, status
			from fx_quote where quote_id = $1 and party_id = $2 for update`, quoteID, partyID).
			Scan(&q.QuoteID, &q.PartyID, &q.SellAsset, &q.BuyAsset, &q.SellAmount, &q.BuyAmount, &q.Mid, &q.Rate, &q.SpreadBps, &q.SpreadMinor, &q.ExpiresAt, &q.Status)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if q.Status != "ISSUED" || !now.Before(q.ExpiresAt) {
			return ErrNotBookable
		}
		if allowance != nil && q.SellAsset == "ZAR" {
			// One booking at a time per party, so two cannot both spend the last of the allowance.
			if _, err := tx.Exec(ctx, `select pg_advisory_xact_lock(hashtext('fx-allowance:' || $1))`, partyID); err != nil {
				return err
			}
			used, err := AllowanceUsed(ctx, tx, partyID, now)
			if err != nil {
				return err
			}
			if used+q.SellAmount > allowance.LimitMinor {
				return ErrAllowance
			}
		}
		if _, err := tx.Exec(ctx, `update fx_quote set status = 'BOOKED' where quote_id = $1`, quoteID); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			insert into fx_deal (deal_id, quote_id, party_id, sell_asset, buy_asset, sell_amount, buy_amount, spread_minor,
			                     sell_account, buy_account, state, updated_at, created_at)
			values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, now(), $12)`,
			dealID, quoteID, partyID, q.SellAsset, q.BuyAsset, q.SellAmount, q.BuyAmount, q.SpreadMinor, sellAcc, buyAcc, StateBooked, now)
		return err
	})
	return q, err
}

// Deal is one booked conversion.
type Deal struct {
	DealID      string
	PartyID     string
	SellAsset   string
	BuyAsset    string
	SellAmount  int64
	BuyAmount   int64
	SpreadMinor int64
	SellAccount string
	BuyAccount  string
	State       string
	Reason      string
	UpdatedAt   time.Time
	SellHolds   []string
	BuyHolds    []string
}

// Deal returns a deal the party holds.
func GetDeal(ctx context.Context, pool *pgxpool.Pool, partyID, dealID string) (Deal, error) {
	var d Deal
	err := pool.QueryRow(ctx, `
		select deal_id, party_id, sell_asset, buy_asset, sell_amount, buy_amount, spread_minor, sell_account, buy_account, state, reason, updated_at
		from fx_deal where deal_id = $1 and party_id = $2`, dealID, partyID).
		Scan(&d.DealID, &d.PartyID, &d.SellAsset, &d.BuyAsset, &d.SellAmount, &d.BuyAmount, &d.SpreadMinor, &d.SellAccount, &d.BuyAccount, &d.State, &d.Reason, &d.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Deal{}, ErrNotFound
	}
	return d, err
}

// OpenDeals returns deals that are not final and were last touched before a time, for the resume loop.
func OpenDeals(ctx context.Context, pool *pgxpool.Pool, before time.Time) ([]string, error) {
	rows, err := pool.Query(ctx, `select deal_id from fx_deal where state not in ('COMPLETED','VOIDED','REVERSED') and updated_at < $1 order by updated_at limit 100`, before.UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// GetDealByID returns a deal for the saga, which works for any party.
func GetDealByID(ctx context.Context, pool *pgxpool.Pool, dealID string) (Deal, error) {
	var d Deal
	err := pool.QueryRow(ctx, `
		select deal_id, party_id, sell_asset, buy_asset, sell_amount, buy_amount, spread_minor, sell_account, buy_account,
		       state, reason, sell_holds, buy_holds, updated_at
		from fx_deal where deal_id = $1`, dealID).
		Scan(&d.DealID, &d.PartyID, &d.SellAsset, &d.BuyAsset, &d.SellAmount, &d.BuyAmount, &d.SpreadMinor,
			&d.SellAccount, &d.BuyAccount, &d.State, &d.Reason, &d.SellHolds, &d.BuyHolds, &d.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Deal{}, ErrNotFound
	}
	return d, err
}

// Advance records a saga step: the new state, the reason, the holds it created, and an optional event, all in one
// transaction. A step that is recorded here is not repeated after a crash.
func Advance(ctx context.Context, pool *pgxpool.Pool, dealID, state, reason string, sellHolds, buyHolds []string, event *outbox.Event) error {
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if sellHolds != nil {
			if _, err := tx.Exec(ctx, `update fx_deal set sell_holds = $2 where deal_id = $1`, dealID, sellHolds); err != nil {
				return err
			}
		}
		if buyHolds != nil {
			if _, err := tx.Exec(ctx, `update fx_deal set buy_holds = $2 where deal_id = $1`, dealID, buyHolds); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `update fx_deal set state = $2, reason = $3, updated_at = now() where deal_id = $1`,
			dealID, state, reason); err != nil {
			return err
		}
		if event != nil {
			return outbox.Append(ctx, tx, *event)
		}
		return nil
	})
}
