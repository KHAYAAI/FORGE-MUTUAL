// Package store holds the bridge's queue, its account cache and its reconciliation runs.
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

// Journal statuses.
const (
	StatusPending = "PENDING"
	StatusPosted  = "POSTED"
	StatusFailed  = "FAILED"
)

// ErrUnknownAccount is returned when an account is not in the cache.
var ErrUnknownAccount = errors.New("store: account not in the cache")

// Open applies the schema. It is idempotent.
func Open(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, schema); err != nil {
		return fmt.Errorf("store: schema: %w", err)
	}
	return nil
}

// AccountInfo is what the bridge knows about a ledger account.
type AccountInfo struct {
	Code  uint16
	Asset string
}

// CachedAccount returns an account's code and asset.
func CachedAccount(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, accountID string) (AccountInfo, error) {
	var code int32
	var a AccountInfo
	err := q.QueryRow(ctx, `select ledger_code, asset from cb_account where account_id = $1`, accountID).Scan(&code, &a.Asset)
	if errors.Is(err, pgx.ErrNoRows) {
		return AccountInfo{}, ErrUnknownAccount
	}
	a.Code = uint16(code)
	return a, err
}

// CacheAccount stores an account from the ledger's directory.
func CacheAccount(ctx context.Context, pool *pgxpool.Pool, accountID string, code uint16, asset string) error {
	_, err := pool.Exec(ctx, `
		insert into cb_account (account_id, ledger_code, asset) values ($1, $2, $3)
		on conflict (account_id) do update set ledger_code = excluded.ledger_code, asset = excluded.asset`,
		accountID, int32(code), asset)
	return err
}

// Queue records one journal inside the caller's transaction. A second record for the same transfer changes nothing.
func Queue(ctx context.Context, tx pgx.Tx, transferID, asset string, amount int64, debit, credit uint16, at time.Time, status, reason string) error {
	_, err := tx.Exec(ctx, `
		insert into cb_journal (transfer_id, asset, amount_minor, debit_code, credit_code, occurred_at, status, last_error)
		values ($1, $2, $3, $4, $5, $6, $7, $8)
		on conflict (transfer_id) do nothing`,
		transferID, asset, amount, int32(debit), int32(credit), at.UTC(), status, reason)
	return err
}

// Job is a queued journal taken by the poster.
type Job struct {
	TransferID string
	Asset      string
	Amount     int64
	DebitCode  uint16
	CreditCode uint16
	OccurredAt time.Time
	Attempts   int
}

// Claim takes up to limit pending journals that have attempts left. Each claim is one attempt.
func Claim(ctx context.Context, pool *pgxpool.Pool, limit, maxAttempts int) ([]Job, error) {
	rows, err := pool.Query(ctx, `
		update cb_journal set attempts = attempts + 1
		where transfer_id in (
		  select transfer_id from cb_journal
		  where status = 'PENDING' and attempts < $2
		  order by created_at for update skip locked limit $1)
		returning transfer_id, asset, amount_minor, debit_code, credit_code, occurred_at, attempts`, limit, maxAttempts)
	if err != nil {
		return nil, fmt.Errorf("store: claim: %w", err)
	}
	defer rows.Close()
	var out []Job
	for rows.Next() {
		var j Job
		var d, c int32
		if err := rows.Scan(&j.TransferID, &j.Asset, &j.Amount, &d, &c, &j.OccurredAt, &j.Attempts); err != nil {
			return nil, err
		}
		j.DebitCode, j.CreditCode = uint16(d), uint16(c)
		out = append(out, j)
	}
	return out, rows.Err()
}

// MarkPosted records the Fineract transaction id.
func MarkPosted(ctx context.Context, pool *pgxpool.Pool, transferID, fineractTxn string) error {
	_, err := pool.Exec(ctx, `
		update cb_journal set status = 'POSTED', fineract_txn = $2, posted_at = now(), last_error = ''
		where transfer_id = $1`, transferID, fineractTxn)
	return err
}

// MarkFailed records a failed attempt. A definitive refusal fails at once. Otherwise the journal stays pending until its
// attempts run out, and then it fails and stays visible.
func MarkFailed(ctx context.Context, pool *pgxpool.Pool, transferID string, maxAttempts int, definitive bool, cause string) error {
	_, err := pool.Exec(ctx, `
		update cb_journal set
		  status = case when $3 or attempts >= $2 then 'FAILED' else 'PENDING' end,
		  last_error = $4
		where transfer_id = $1`, transferID, maxAttempts, definitive, cause)
	return err
}

// PendingCount returns how many journals are waiting to be posted.
func PendingCount(ctx context.Context, pool *pgxpool.Pool) (int, error) {
	var n int
	err := pool.QueryRow(ctx, `select count(*) from cb_journal where status = 'PENDING'`).Scan(&n)
	return n, err
}

// Journal returns one journal's status, for tests and operators.
func Journal(ctx context.Context, pool *pgxpool.Pool, transferID string) (status, fineractTxn, lastErr string, err error) {
	err = pool.QueryRow(ctx, `select status, fineract_txn, last_error from cb_journal where transfer_id = $1`, transferID).
		Scan(&status, &fineractTxn, &lastErr)
	return
}

// RecordRun stores one reconciliation run.
func RecordRun(ctx context.Context, pool *pgxpool.Pool, at time.Time, checked, breaks int, skipped, detail string) error {
	_, err := pool.Exec(ctx, `insert into cb_recon_run (finished_at, checked, breaks, skipped, detail) values ($1, $2, $3, $4, $5)`,
		at.UTC(), checked, breaks, skipped, detail)
	return err
}
