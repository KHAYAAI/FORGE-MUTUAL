// Package store is the payments database: one row per payment, and the outbox.
package store

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/khayaai/forge-mutual/pkg/outbox"
)

//go:embed schema.sql
var schema string

// ErrNotFound is returned for an unknown payment.
var ErrNotFound = errors.New("store: not found")

// Row is one payment as stored.
type Row struct {
	PaymentID         string
	PartyID           string
	FromAccountID     string
	Asset             string
	AmountMinor       int64
	State             string
	Reason            string
	RailStatus        string
	ReserveTransferID string
	InitiatedBy       string
	CreatedAt         time.Time
}

// Store wraps the pool.
type Store struct{ pool *pgxpool.Pool }

// Querier is satisfied by *pgxpool.Pool and pgx.Tx.
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// Open connects and applies the schema and the outbox table.
func Open(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	if _, err := pool.Exec(ctx, schema); err != nil {
		pool.Close()
		return nil, fmt.Errorf("store: schema: %w", err)
	}
	if err := outbox.Migrate(ctx, pool); err != nil {
		pool.Close()
		return nil, err
	}
	return &Store{pool: pool}, nil
}

// Close releases the pool.
func (s *Store) Close() { s.pool.Close() }

// Pool exposes the pool for the outbox relay and readiness checks.
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

// WithTx runs fn in one transaction.
func (s *Store) WithTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Upsert writes the row's current state. An empty reserve transfer id keeps the stored one.
func Upsert(ctx context.Context, tx pgx.Tx, r Row) error {
	_, err := tx.Exec(ctx, `
		insert into payment (payment_id, party_id, from_account_id, asset, amount_minor, state, reason,
			rail_status, reserve_transfer_id, initiated_by)
		values ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, nullif($9, ''), $10)
		on conflict (payment_id) do update set
			state = excluded.state,
			reason = excluded.reason,
			rail_status = excluded.rail_status,
			reserve_transfer_id = coalesce(excluded.reserve_transfer_id, payment.reserve_transfer_id),
			updated_at = now()`,
		r.PaymentID, r.PartyID, r.FromAccountID, r.Asset, r.AmountMinor, r.State, r.Reason, r.RailStatus,
		r.ReserveTransferID, r.InitiatedBy)
	return err
}

// Get reads one payment.
func Get(ctx context.Context, q Querier, id string) (Row, error) {
	var r Row
	var transfer *string
	err := q.QueryRow(ctx, `select payment_id::text, party_id::text, from_account_id, asset, amount_minor, state,
		reason, rail_status, reserve_transfer_id, initiated_by, created_at from payment where payment_id = $1::uuid`, id).
		Scan(&r.PaymentID, &r.PartyID, &r.FromAccountID, &r.Asset, &r.AmountMinor, &r.State, &r.Reason,
			&r.RailStatus, &transfer, &r.InitiatedBy, &r.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Row{}, ErrNotFound
	}
	if transfer != nil {
		r.ReserveTransferID = *transfer
	}
	return r, err
}

// HeldSince returns the total of a party's payments in an asset that are on hold or settled since the
// given time. Policy reads it as the day's velocity.
func HeldSince(ctx context.Context, q Querier, partyID, asset string, since time.Time) (int64, error) {
	var total int64
	err := q.QueryRow(ctx, `select coalesce(sum(amount_minor), 0)::bigint from payment
		where party_id = $1::uuid and asset = $2 and created_at >= $3
		  and state in ('RESERVED', 'SETTLED', 'REPAIR_REQUIRED')`, partyID, asset, since).Scan(&total)
	return total, err
}

// IsUnique reports whether err is a unique-constraint violation.
func IsUnique(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505"
}
