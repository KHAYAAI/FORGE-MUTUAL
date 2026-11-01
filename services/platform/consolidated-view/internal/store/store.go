// Package store holds the consolidated read model. Projection is idempotent at the data level: a transfer is counted
// once per account, however many times it arrives.
package store

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var schema string

// ErrNotFound is returned for an account the party does not own. It is the same answer as for an account that does
// not exist, so ownership is not revealed.
var ErrNotFound = errors.New("store: not found")

// Transfer is one committed transfer, as the projection needs it.
type Transfer struct {
	ID      string
	Asset   string
	Debit   string
	Credit  string
	Amount  int64
	Code    uint32
	Pending bool
	At      time.Time
}

// Store is the read model.
type Store struct {
	pool *pgxpool.Pool
}

// Open applies the schema. It is idempotent.
func Open(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, schema); err != nil {
		return fmt.Errorf("store: schema: %w", err)
	}
	return nil
}

// New returns a store over pool.
func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Pool exposes the pool for health checks and tests.
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

// Project applies one transfer inside the caller's transaction. A pending transfer is a hold and is not posted, so it
// changes nothing here. A transfer already recorded for an account changes nothing either.
func Project(ctx context.Context, tx pgx.Tx, t Transfer) error {
	if t.Pending || t.Amount <= 0 {
		return nil
	}
	for _, leg := range []struct {
		account, direction string
	}{{t.Debit, "OUT"}, {t.Credit, "IN"}} {
		if _, err := tx.Exec(ctx, `
			insert into cv_account (account_id, entity, asset) values ($1, 'BANK', $2)
			on conflict (account_id) do nothing`, leg.account, t.Asset); err != nil {
			return fmt.Errorf("store: account: %w", err)
		}
		var seq int64
		err := tx.QueryRow(ctx, `
			insert into cv_activity (account_id, transfer_id, direction, amount_minor, asset, code, at)
			values ($1, $2, $3, $4, $5, $6, $7)
			on conflict (account_id, transfer_id, direction) do nothing
			returning seq`,
			leg.account, t.ID, leg.direction, t.Amount, t.Asset, int32(t.Code), t.At.UTC()).Scan(&seq)
		if errors.Is(err, pgx.ErrNoRows) {
			continue // already counted for this account
		}
		if err != nil {
			return fmt.Errorf("store: activity: %w", err)
		}
		debit, credit := int64(0), int64(0)
		if leg.direction == "OUT" {
			debit = t.Amount
		} else {
			credit = t.Amount
		}
		if _, err := tx.Exec(ctx, `
			insert into cv_balance (account_id, debit_minor, credit_minor, source) values ($1, $2, $3, 'EVENT')
			on conflict (account_id) do update set
			  debit_minor = cv_balance.debit_minor + excluded.debit_minor,
			  credit_minor = cv_balance.credit_minor + excluded.credit_minor,
			  source = 'EVENT', as_of = now()`, leg.account, debit, credit); err != nil {
			return fmt.Errorf("store: balance: %w", err)
		}
	}
	return nil
}

// AccountView is one owned account with its balance.
type AccountView struct {
	AccountID string
	Asset     string
	Available int64 // credits minus debits, posted
	Insured   bool
	ProductID string
	AsOf      time.Time
	Source    string
}

// Balances returns the accounts a party owns. Accounts without a reconciled owner are not returned.
func (s *Store) Balances(ctx context.Context, partyID string) ([]AccountView, error) {
	if partyID == "" {
		return nil, errors.New("store: party is required")
	}
	rows, err := s.pool.Query(ctx, `
		select a.account_id, a.asset, coalesce(b.credit_minor - b.debit_minor, 0), a.insured, a.product_id,
		       coalesce(b.as_of, a.seen_at), coalesce(b.source, 'EVENT')
		from cv_account a left join cv_balance b on b.account_id = a.account_id
		where a.owner_party_id = $1
		order by a.account_id`, partyID)
	if err != nil {
		return nil, fmt.Errorf("store: balances: %w", err)
	}
	defer rows.Close()
	var out []AccountView
	for rows.Next() {
		var v AccountView
		if err := rows.Scan(&v.AccountID, &v.Asset, &v.Available, &v.Insured, &v.ProductID, &v.AsOf, &v.Source); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// ActivityRow is one movement on an account.
type ActivityRow struct {
	Seq        uint64
	TransferID string
	Direction  string
	Amount     int64
	Asset      string
	Code       uint32
	At         time.Time
}

// Activity returns movements on an account the party owns, after a sequence number. It returns ErrNotFound for any
// other account.
func (s *Store) Activity(ctx context.Context, partyID, accountID string, after uint64, limit int) ([]ActivityRow, error) {
	var owner string
	err := s.pool.QueryRow(ctx, `select owner_party_id from cv_account where account_id = $1`, accountID).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (owner == "" || owner != partyID)) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: owner: %w", err)
	}
	rows, err := s.pool.Query(ctx, `
		select seq, transfer_id, direction, amount_minor, asset, code, at from cv_activity
		where account_id = $1 and seq > $2 order by seq limit $3`, accountID, int64(after), limit)
	if err != nil {
		return nil, fmt.Errorf("store: activity: %w", err)
	}
	defer rows.Close()
	var out []ActivityRow
	for rows.Next() {
		var r ActivityRow
		var seq int64
		var code int32
		if err := rows.Scan(&seq, &r.TransferID, &r.Direction, &r.Amount, &r.Asset, &code, &r.At); err != nil {
			return nil, err
		}
		r.Seq, r.Code = uint64(seq), uint32(code)
		out = append(out, r)
	}
	return out, rows.Err()
}

// LastRun returns the time and counts of the latest reconciliation, or a zero time if none has run.
func (s *Store) LastRun(ctx context.Context) (time.Time, int, int, error) {
	var at time.Time
	var checked, drifted int
	err := s.pool.QueryRow(ctx, `select finished_at, accounts_checked, drifted from cv_recon_run order by run_id desc limit 1`).
		Scan(&at, &checked, &drifted)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, 0, 0, nil
	}
	return at, checked, drifted, err
}

// SetLedgerBalance makes the stored balance equal the ledger's posted figures. It reports whether the stored figure
// differed, which is drift: the projection had missed or double-counted something.
func (s *Store) SetLedgerBalance(ctx context.Context, accountID string, debit, credit int64) (bool, error) {
	var cd, cc int64
	found := true
	err := s.pool.QueryRow(ctx, `select debit_minor, credit_minor from cv_balance where account_id = $1`, accountID).Scan(&cd, &cc)
	if errors.Is(err, pgx.ErrNoRows) {
		found = false
	} else if err != nil {
		return false, fmt.Errorf("store: read balance: %w", err)
	}
	drifted := (!found && (debit != 0 || credit != 0)) || (found && (cd != debit || cc != credit))
	if _, err := s.pool.Exec(ctx, `
		insert into cv_balance (account_id, debit_minor, credit_minor, source) values ($1, $2, $3, 'LEDGER')
		on conflict (account_id) do update set
		  debit_minor = excluded.debit_minor, credit_minor = excluded.credit_minor,
		  source = 'LEDGER', as_of = now()`, accountID, debit, credit); err != nil {
		return false, fmt.Errorf("store: set balance: %w", err)
	}
	return drifted, nil
}
