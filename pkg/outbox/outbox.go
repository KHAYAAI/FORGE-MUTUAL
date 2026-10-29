// Package outbox is a transactional outbox for Postgres. A service appends events in the same
// transaction as the change they describe, and a relay publishes them at least once. Consumers
// dedupe by EventID (ARCHITECTURE.md 7.8, 18.2).
package outbox

import (
	"context"
	"crypto/rand"
	_ "embed"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var schema string

// Event is one message to publish.
type Event struct {
	EventID string
	Topic   string
	Key     []byte
	Payload []byte
}

// Publisher sends one message and returns once the broker has accepted it.
type Publisher interface {
	Publish(ctx context.Context, topic string, key, value []byte) error
}

// Migrate creates the outbox table if it does not exist.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, schema); err != nil {
		return fmt.Errorf("outbox: migrate: %w", err)
	}
	return nil
}

// Append writes events inside the caller's transaction.
func Append(ctx context.Context, tx pgx.Tx, events ...Event) error {
	for _, e := range events {
		if _, err := tx.Exec(ctx,
			`insert into outbox (event_id, topic, event_key, payload) values ($1, $2, $3, $4)`,
			e.EventID, e.Topic, e.Key, e.Payload); err != nil {
			return fmt.Errorf("outbox: append: %w", err)
		}
	}
	return nil
}

// Relay publishes up to limit unpublished events and marks them published only if every
// publish succeeded, in one transaction. Rows are locked with SKIP LOCKED, so several relays
// can run at once. It returns the number published.
func Relay(ctx context.Context, pool *pgxpool.Pool, limit int, pub Publisher) (int, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("outbox: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after commit

	rows, err := tx.Query(ctx, `
		select seq, event_id::text, topic, event_key, payload from outbox
		where published_at is null order by seq limit $1 for update skip locked`, limit)
	if err != nil {
		return 0, fmt.Errorf("outbox: select: %w", err)
	}
	var seqs []int64
	var events []Event
	for rows.Next() {
		var seq int64
		var e Event
		if err := rows.Scan(&seq, &e.EventID, &e.Topic, &e.Key, &e.Payload); err != nil {
			rows.Close()
			return 0, fmt.Errorf("outbox: scan: %w", err)
		}
		seqs = append(seqs, seq)
		events = append(events, e)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("outbox: rows: %w", err)
	}
	if len(events) == 0 {
		return 0, nil
	}
	for _, e := range events {
		if err := pub.Publish(ctx, e.Topic, e.Key, e.Payload); err != nil {
			return 0, fmt.Errorf("outbox: publish %s: %w", e.EventID, err)
		}
	}
	if _, err := tx.Exec(ctx, `update outbox set published_at = now() where seq = any($1)`, seqs); err != nil {
		return 0, fmt.Errorf("outbox: mark: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("outbox: commit: %w", err)
	}
	return len(events), nil
}

// NewEventID returns a random RFC 4122 version 4 UUID.
func NewEventID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}
