// Package store holds payout batches and their items. A batch moves through its states; an item's state moves forward
// only. A settled or failed item is final: nothing sets it back to be sent again.
package store

import (
	"context"
	_ "embed"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var schema string

// ErrNotFound is returned when a batch or an item does not exist.
var ErrNotFound = errors.New("store: not found")

// Batch states.
const (
	StateDraft      = "DRAFT"
	StateRunning    = "RUNNING"
	StateCompleted  = "COMPLETED"
	StateExceptions = "COMPLETED_WITH_EXCEPTIONS"
)

// Item states.
const (
	ItemPending   = "PENDING"
	ItemSent      = "SENT"
	ItemSettled   = "SETTLED"
	ItemFailed    = "FAILED"
	ItemUncertain = "UNCERTAIN"
)

// Batch is a payout batch. Items are present only when a batch is loaded whole.
type Batch struct {
	ID             string
	IdempotencyKey string
	RequestHash    []byte
	PartyID        string
	FromAccountID  string
	Asset          string
	TotalMinor     int64
	State          string
	CreatedBy      string
	ApprovedBy     string
	Run            uint32
	Items          []Item
}

// Item is one payment in a batch.
type Item struct {
	Key             string
	Seq             int
	AmountMinor     int64
	BeneficiaryName string
	AccountNumber   string
	BankCode        string
	Reference       string
	State           string
	PaymentID       string
	Reason          string
}

// Open applies the schema. It is idempotent.
func Open(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, schema); err != nil {
		return fmt.Errorf("store: schema: %w", err)
	}
	return nil
}

type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

const headerCols = `batch_id::text, idempotency_key, request_hash, party_id::text, from_account_id, asset, total_minor,
	state, created_by, approved_by, run`

func scanHeader(row pgx.Row) (Batch, error) {
	var b Batch
	var run int32
	err := row.Scan(&b.ID, &b.IdempotencyKey, &b.RequestHash, &b.PartyID, &b.FromAccountID, &b.Asset, &b.TotalMinor,
		&b.State, &b.CreatedBy, &b.ApprovedBy, &run)
	if errors.Is(err, pgx.ErrNoRows) {
		return Batch{}, ErrNotFound
	}
	if err != nil {
		return Batch{}, err
	}
	b.Run = uint32(run)
	return b, nil
}

func loadItems(ctx context.Context, q querier, batchID string, openOnly bool) ([]Item, error) {
	query := `select item_key, seq, amount_minor, beneficiary_name, account_number, bank_code, reference, state,
		payment_id, reason from payout_item where batch_id = $1::uuid`
	if openOnly {
		query += ` and state in ('PENDING', 'SENT', 'UNCERTAIN')`
	}
	query += ` order by seq`
	rows, err := q.Query(ctx, query, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Item
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.Key, &it.Seq, &it.AmountMinor, &it.BeneficiaryName, &it.AccountNumber, &it.BankCode,
			&it.Reference, &it.State, &it.PaymentID, &it.Reason); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// Create stores a new batch with its items. A key used before returns the batch already stored, and the caller compares
// the request hash. The bool is true when this call created the batch.
func Create(ctx context.Context, pool *pgxpool.Pool, b Batch) (Batch, bool, error) {
	var out Batch
	created := false
	err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		var id string
		err := tx.QueryRow(ctx, `insert into payout_batch (batch_id, idempotency_key, request_hash, party_id, from_account_id,
			asset, total_minor, state, created_by)
			values ($1::uuid, $2, $3, $4::uuid, $5, $6, $7, 'DRAFT', $8)
			on conflict (idempotency_key) do nothing
			returning batch_id::text`,
			b.ID, b.IdempotencyKey, b.RequestHash, b.PartyID, b.FromAccountID, b.Asset, b.TotalMinor, b.CreatedBy).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			existing, err := scanHeader(tx.QueryRow(ctx, `select `+headerCols+` from payout_batch where idempotency_key = $1`,
				b.IdempotencyKey))
			if err != nil {
				return err
			}
			existing.Items, err = loadItems(ctx, tx, existing.ID, false)
			out = existing
			return err
		}
		if err != nil {
			return err
		}
		for i, it := range b.Items {
			if _, err := tx.Exec(ctx, `insert into payout_item (batch_id, item_key, seq, amount_minor, beneficiary_name,
				account_number, bank_code, reference, state)
				values ($1::uuid, $2, $3, $4, $5, $6, $7, $8, 'PENDING')`,
				id, it.Key, i+1, it.AmountMinor, it.BeneficiaryName, it.AccountNumber, it.BankCode, it.Reference); err != nil {
				return err
			}
		}
		created = true
		out, err = scanHeader(tx.QueryRow(ctx, `select `+headerCols+` from payout_batch where batch_id = $1::uuid`, id))
		if err != nil {
			return err
		}
		out.Items, err = loadItems(ctx, tx, id, false)
		return err
	})
	return out, created, err
}

// Get returns a batch with its items.
func Get(ctx context.Context, pool *pgxpool.Pool, id string) (Batch, error) {
	b, err := Header(ctx, pool, id)
	if err != nil {
		return Batch{}, err
	}
	b.Items, err = loadItems(ctx, pool, id, false)
	return b, err
}

// Header returns a batch without its items.
func Header(ctx context.Context, pool *pgxpool.Pool, id string) (Batch, error) {
	return scanHeader(pool.QueryRow(ctx, `select `+headerCols+` from payout_batch where batch_id = $1::uuid`, id))
}

// Lock reads a batch header under a row lock, for the rest of the transaction.
func Lock(ctx context.Context, tx pgx.Tx, id string) (Batch, error) {
	return scanHeader(tx.QueryRow(ctx, `select `+headerCols+` from payout_batch where batch_id = $1::uuid for update`, id))
}

// Start moves a batch to RUNNING for a run. A checker is recorded when one is given.
func Start(ctx context.Context, tx pgx.Tx, id string, run uint32, checker string) error {
	_, err := tx.Exec(ctx, `update payout_batch
		set state = 'RUNNING', run = $2, approved_by = case when $3 = '' then approved_by else $3 end, updated_at = now()
		where batch_id = $1::uuid`, id, int32(run), checker)
	return err
}

// GetItem returns one item of a batch.
func GetItem(ctx context.Context, pool *pgxpool.Pool, batchID, key string) (Item, error) {
	var it Item
	err := pool.QueryRow(ctx, `select item_key, seq, amount_minor, beneficiary_name, account_number, bank_code, reference,
		state, payment_id, reason from payout_item where batch_id = $1::uuid and item_key = $2`, batchID, key).Scan(
		&it.Key, &it.Seq, &it.AmountMinor, &it.BeneficiaryName, &it.AccountNumber, &it.BankCode, &it.Reference,
		&it.State, &it.PaymentID, &it.Reason)
	if errors.Is(err, pgx.ErrNoRows) {
		return Item{}, ErrNotFound
	}
	return it, err
}

// OpenItems returns the items of a batch that have no final outcome yet.
func OpenItems(ctx context.Context, pool *pgxpool.Pool, batchID string) ([]Item, error) {
	return loadItems(ctx, pool, batchID, true)
}

// SetItem moves an item forward. An item that is already settled or failed is left as it is. An empty payment id keeps
// the one already recorded.
func SetItem(ctx context.Context, pool *pgxpool.Pool, batchID, key, state, paymentID, reason string) error {
	_, err := pool.Exec(ctx, `update payout_item
		set state = $3, payment_id = case when $4 = '' then payment_id else $4 end, reason = $5, updated_at = now()
		where batch_id = $1::uuid and item_key = $2 and state in ('PENDING', 'SENT', 'UNCERTAIN')`,
		batchID, key, state, paymentID, reason)
	return err
}

// FinishRun sets the state a run leaves the batch in: COMPLETED when every item settled, otherwise
// COMPLETED_WITH_EXCEPTIONS. A run that is no longer the current one changes nothing, and the state read back is returned.
func FinishRun(ctx context.Context, pool *pgxpool.Pool, batchID string, run uint32) (string, error) {
	var state string
	err := pool.QueryRow(ctx, `update payout_batch
		set state = case when exists (select 1 from payout_item where batch_id = $1::uuid and state <> 'SETTLED')
			then 'COMPLETED_WITH_EXCEPTIONS' else 'COMPLETED' end,
			updated_at = now()
		where batch_id = $1::uuid and run = $2 and state = 'RUNNING'
		returning state`, batchID, int32(run)).Scan(&state)
	if errors.Is(err, pgx.ErrNoRows) {
		b, err := Header(ctx, pool, batchID)
		return b.State, err
	}
	return state, err
}
