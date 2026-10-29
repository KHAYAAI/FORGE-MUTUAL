// Package postgres implements gateway.Repository on PostgreSQL 17 (pgx v5).
package postgres

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/khayaai/forge-mutual/services/bank/ledger-gateway-bank/internal/gateway"
	"github.com/khayaai/forge-mutual/services/bank/ledger-gateway-bank/internal/ledger"
)

//go:embed schema.sql
var schema string

// Repo is the PostgreSQL-backed repository.
type Repo struct {
	pool *pgxpool.Pool
}

var _ gateway.Repository = (*Repo)(nil)

// Connect opens a pool and applies the schema.
func Connect(ctx context.Context, dsn string) (*Repo, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres: connect: %w", err)
	}
	if _, err := pool.Exec(ctx, schema); err != nil {
		pool.Close()
		return nil, fmt.Errorf("postgres: schema: %w", err)
	}
	return &Repo{pool: pool}, nil
}

// Close releases the pool.
func (r *Repo) Close() { r.pool.Close() }

func idStrings(ids []ledger.ID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return out
}

func parseIDs(ss []string) ([]ledger.ID, error) {
	out := make([]ledger.ID, len(ss))
	for i, s := range ss {
		id, err := ledger.ParseID(s)
		if err != nil {
			return nil, err
		}
		out[i] = id
	}
	return out, nil
}

// Begin implements gateway.Repository.
func (r *Repo) Begin(ctx context.Context, caller, key string, hash []byte, proposed []ledger.ID) (gateway.Record, bool, error) {
	tag, err := r.pool.Exec(ctx, `
		insert into idempotency_record (caller, idempotency_key, request_hash, status, ledger_ids)
		values ($1, $2, $3, 'PENDING', $4)
		on conflict (caller, idempotency_key) do nothing`,
		caller, key, hash, idStrings(proposed))
	if err != nil {
		return gateway.Record{}, false, fmt.Errorf("postgres: begin: %w", err)
	}
	created := tag.RowsAffected() == 1

	var (
		status string
		stored []byte
		ids    []string
		result []byte
	)
	err = r.pool.QueryRow(ctx, `
		select status, request_hash, ledger_ids, result
		from idempotency_record where caller = $1 and idempotency_key = $2`,
		caller, key).Scan(&status, &stored, &ids, &result)
	if err != nil {
		return gateway.Record{}, false, fmt.Errorf("postgres: read record: %w", err)
	}
	if !bytes.Equal(stored, hash) {
		return gateway.Record{}, false, gateway.ErrConflict
	}
	parsed, err := parseIDs(ids)
	if err != nil {
		return gateway.Record{}, false, err
	}
	return gateway.Record{Status: gateway.RecordStatus(status), Hash: stored, IDs: parsed, Result: result}, created, nil
}

// Complete implements gateway.Repository.
func (r *Repo) Complete(ctx context.Context, caller, key string, result []byte, dir []gateway.DirectoryEntry, events []gateway.OutboxEvent) error {
	return r.inTx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			update idempotency_record set status = 'COMMITTED', result = $3, updated_at = now()
			where caller = $1 and idempotency_key = $2 and status = 'PENDING'`,
			caller, key, result)
		if err != nil {
			return fmt.Errorf("postgres: complete record: %w", err)
		}
		if tag.RowsAffected() == 0 {
			var status string
			if err := tx.QueryRow(ctx, `select status from idempotency_record where caller = $1 and idempotency_key = $2`,
				caller, key).Scan(&status); err != nil {
				return fmt.Errorf("postgres: complete: no record: %w", err)
			}
			if status == string(gateway.StatusCommitted) {
				return nil
			}
			return fmt.Errorf("postgres: complete: unexpected status %s", status)
		}
		for _, d := range dir {
			if _, err := tx.Exec(ctx, `
				insert into account_directory (account_id, entity_code, owner_party_id, asset, ledger, code, product_id, insured)
				values ($1, $2, nullif($3, ''), $4, $5, $6, nullif($7, ''), $8)
				on conflict (account_id) do nothing`,
				d.AccountID.String(), d.Entity, d.OwnerPartyID, d.Asset, int64(d.Ledger), int32(d.Code), d.ProductID, d.Insured); err != nil {
				return fmt.Errorf("postgres: directory: %w", err)
			}
		}
		for _, e := range events {
			if _, err := tx.Exec(ctx, `
				insert into outbox (event_id, topic, event_key, payload) values ($1, $2, $3, $4)`,
				e.EventID, e.Topic, e.Key, e.Payload); err != nil {
				return fmt.Errorf("postgres: outbox: %w", err)
			}
		}
		return nil
	})
}

// Directory implements gateway.Repository.
func (r *Repo) Directory(ctx context.Context, ids []ledger.ID) (map[ledger.ID]gateway.DirectoryEntry, error) {
	out := make(map[ledger.ID]gateway.DirectoryEntry, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx, `
		select account_id, entity_code, coalesce(owner_party_id, ''), asset, ledger, code, coalesce(product_id, ''), insured
		from account_directory where account_id = any($1)`, idStrings(ids))
	if err != nil {
		return nil, fmt.Errorf("postgres: directory: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			idStr, entity, owner, asset, product string
			ledgerNo                             int64
			code                                 int32
			insured                              bool
		)
		if err := rows.Scan(&idStr, &entity, &owner, &asset, &ledgerNo, &code, &product, &insured); err != nil {
			return nil, fmt.Errorf("postgres: scan directory: %w", err)
		}
		id, err := ledger.ParseID(idStr)
		if err != nil {
			return nil, err
		}
		out[id] = gateway.DirectoryEntry{
			AccountID: id, Entity: entity, Asset: asset, Ledger: uint32(ledgerNo), Code: uint16(code),
			OwnerPartyID: owner, ProductID: product, Insured: insured,
		}
	}
	return out, rows.Err()
}

// RelayBatch implements gateway.Repository. Rows are locked with SKIP LOCKED so several
// relays can run at once without publishing the same event twice in parallel.
func (r *Repo) RelayBatch(ctx context.Context, limit int, publish func([]gateway.OutboxEvent) error) (int, error) {
	var n int
	err := r.inTx(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			select seq, event_id::text, topic, event_key, payload from outbox
			where published_at is null order by seq limit $1 for update skip locked`, limit)
		if err != nil {
			return fmt.Errorf("postgres: select outbox: %w", err)
		}
		var events []gateway.OutboxEvent
		var seqs []int64
		for rows.Next() {
			var seq int64
			var e gateway.OutboxEvent
			if err := rows.Scan(&seq, &e.EventID, &e.Topic, &e.Key, &e.Payload); err != nil {
				rows.Close()
				return fmt.Errorf("postgres: scan outbox: %w", err)
			}
			seqs = append(seqs, seq)
			events = append(events, e)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		if len(events) == 0 {
			return nil
		}
		if err := publish(events); err != nil {
			return fmt.Errorf("postgres: publish: %w", err)
		}
		if _, err := tx.Exec(ctx, `update outbox set published_at = now() where seq = any($1)`, seqs); err != nil {
			return fmt.Errorf("postgres: mark published: %w", err)
		}
		n = len(events)
		return nil
	})
	return n, err
}

func (r *Repo) inTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres: begin tx: %w", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("postgres: commit: %w", err)
	}
	return nil
}

// Ping checks the database connection, for readiness probes.
func (r *Repo) Ping(ctx context.Context) error { return r.pool.Ping(ctx) }
