// Package store appends to and reads the audit chain in Postgres.
package store

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/khayaai/forge-mutual/services/shared/audit-service/internal/chain"
)

//go:embed schema.sql
var schema string

// chainLock serialises appends across every writer, so two entries can never take the same sequence number or link to
// the same predecessor.
const chainLock = 0x46524745 // "FRGE"

// Store is the audit database.
type Store struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

// Open connects and applies the schema. It is idempotent.
func Open(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("store: connect: %w", err)
	}
	// Several processes may start at once. The schema is applied under a lock so their DDL cannot race.
	if err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `select pg_advisory_xact_lock($1)`, chainLock+1); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, schema)
		return err
	}); err != nil {
		pool.Close()
		return nil, fmt.Errorf("store: schema: %w", err)
	}
	return &Store{pool: pool, now: time.Now}, nil
}

// Close releases the pool.
func (s *Store) Close() { s.pool.Close() }

// Pool exposes the pool to health checks and tests in this module.
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

// SetClock replaces the clock. Tests use it.
func (s *Store) SetClock(now func() time.Time) { s.now = now }

// Append validates and writes one entry at the end of the chain, and returns it with its hashes.
func (s *Store) Append(ctx context.Context, in chain.Input) (chain.Entry, error) {
	if err := in.Validate(); err != nil {
		return chain.Entry{}, err
	}
	var out chain.Entry
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `select pg_advisory_xact_lock($1)`, chainLock); err != nil {
			return fmt.Errorf("store: lock: %w", err)
		}
		prev, seq, err := head(ctx, tx)
		if err != nil {
			return err
		}
		e := chain.Entry{
			Seq: seq + 1, At: chain.Micros(s.now()), Actor: in.Actor, Action: in.Action,
			Entity: in.Entity, Payload: in.Payload, PrevHash: prev,
		}
		chain.Seal(&e)
		if _, err := tx.Exec(ctx, `
			insert into audit_entry (seq, at_micros, actor, action, entity, payload, payload_hash, prev_hash, hash)
			values ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			int64(e.Seq), e.At.UnixMicro(), e.Actor, e.Action, e.Entity, e.Payload,
			e.PayloadHash[:], e.PrevHash[:], e.Hash[:]); err != nil {
			return fmt.Errorf("store: insert: %w", err)
		}
		out = e
		return nil
	})
	return out, err
}

// Head returns the last sequence number and its hash. An empty log has sequence zero and the genesis hash.
func (s *Store) Head(ctx context.Context) (uint64, [32]byte, error) {
	prev, seq, err := head(ctx, s.pool)
	return seq, prev, err
}

type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func head(ctx context.Context, q querier) ([32]byte, uint64, error) {
	var (
		seq  int64
		hash []byte
	)
	err := q.QueryRow(ctx, `select seq, hash from audit_entry order by seq desc limit 1`).Scan(&seq, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return chain.Genesis, 0, nil
	}
	if err != nil {
		return [32]byte{}, 0, fmt.Errorf("store: head: %w", err)
	}
	var h [32]byte
	copy(h[:], hash)
	return h, uint64(seq), nil
}

// Range returns up to limit entries with sequence after afterSeq, optionally for one entity, in sequence order.
func (s *Store) Range(ctx context.Context, entity string, afterSeq uint64, limit int) ([]chain.Entry, error) {
	rows, err := s.pool.Query(ctx, `
		select seq, at_micros, actor, action, entity, payload, prev_hash, hash
		from audit_entry
		where seq > $1 and ($2::text = '' or entity = $2)
		order by seq limit $3`, int64(afterSeq), entity, limit)
	if err != nil {
		return nil, fmt.Errorf("store: range: %w", err)
	}
	defer rows.Close()
	var out []chain.Entry
	for rows.Next() {
		var (
			seq, micros         int64
			actor, action, ent  string
			payload, prev, hash []byte
		)
		if err := rows.Scan(&seq, &micros, &actor, &action, &ent, &payload, &prev, &hash); err != nil {
			return nil, fmt.Errorf("store: scan: %w", err)
		}
		e := chain.Entry{
			Seq: uint64(seq), At: time.UnixMicro(micros).UTC(), Actor: actor, Action: action, Entity: ent,
			Payload: append([]byte(nil), payload...),
		}
		copy(e.PrevHash[:], prev)
		copy(e.Hash[:], hash)
		e.PayloadHash = chain.PayloadHash(e.Payload)
		out = append(out, e)
	}
	return out, rows.Err()
}

// VerifyAll walks the whole chain in batches and checks every link and hash. It returns the number of entries and
// the head hash it verified to.
func (s *Store) VerifyAll(ctx context.Context) (uint64, [32]byte, error) {
	prev := chain.Genesis
	var after, count uint64
	for {
		batch, err := s.Range(ctx, "", after, 500)
		if err != nil {
			return 0, [32]byte{}, err
		}
		if len(batch) == 0 {
			return count, prev, nil
		}
		prev, err = chain.Verify(prev, after+1, batch)
		if err != nil {
			return 0, [32]byte{}, err
		}
		after = batch[len(batch)-1].Seq
		count += uint64(len(batch))
	}
}

// HashAt returns the stored hash of one sequence number, for checking anchors.
func (s *Store) HashAt(ctx context.Context, seq uint64) ([32]byte, error) {
	var hash []byte
	err := s.pool.QueryRow(ctx, `select hash from audit_entry where seq = $1`, int64(seq)).Scan(&hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return [32]byte{}, ErrNotFound
	}
	if err != nil {
		return [32]byte{}, fmt.Errorf("store: hash at: %w", err)
	}
	var h [32]byte
	copy(h[:], hash)
	return h, nil
}

// ErrNotFound is returned when a sequence number does not exist.
var ErrNotFound = errors.New("store: not found")
