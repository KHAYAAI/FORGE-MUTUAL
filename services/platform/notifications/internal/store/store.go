// Package store holds contacts and messages.
package store

import (
	"context"
	_ "embed"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/khayaai/forge-mutual/services/platform/notifications/internal/templates"
)

//go:embed schema.sql
var schema string

// ErrNoContact is returned when a party has no address on a channel.
var ErrNoContact = errors.New("store: no contact on file for this channel")

// Message statuses.
const (
	StatusPending = "PENDING"
	StatusSent    = "SENT"
	StatusFailed  = "FAILED"
	StatusSkipped = "SKIPPED"
)

// Open applies the schema. It is idempotent.
func Open(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, schema); err != nil {
		return fmt.Errorf("store: schema: %w", err)
	}
	return nil
}

// SetContact stores a sealed address for a party and channel, replacing any earlier one.
func SetContact(ctx context.Context, pool *pgxpool.Pool, partyID, channel string, sealed []byte) error {
	_, err := pool.Exec(ctx, `
		insert into notif_contact (party_id, channel, address_sealed) values ($1, $2, $3)
		on conflict (party_id, channel) do update set address_sealed = excluded.address_sealed, updated_at = now()`,
		partyID, channel, sealed)
	return err
}

// Contact returns the sealed address for a party and channel.
func Contact(ctx context.Context, pool *pgxpool.Pool, partyID, channel string) ([]byte, error) {
	var sealed []byte
	err := pool.QueryRow(ctx, `select address_sealed from notif_contact where party_id = $1 and channel = $2`,
		partyID, channel).Scan(&sealed)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoContact
	}
	return sealed, err
}

// Enqueue records one message inside the caller's transaction. A party with no contact on the channel gets a SKIPPED
// row with the reason, so the gap is visible. A second message for the same event and channel changes nothing.
func Enqueue(ctx context.Context, tx pgx.Tx, eventID, messageID string, m templates.Message) error {
	var hasContact bool
	if err := tx.QueryRow(ctx, `select exists(select 1 from notif_contact where party_id = $1 and channel = $2)`,
		m.PartyID, m.Channel).Scan(&hasContact); err != nil {
		return fmt.Errorf("store: contact check: %w", err)
	}
	status, lastErr := StatusPending, ""
	if !hasContact {
		status, lastErr = StatusSkipped, "no contact on file for this channel"
	}
	_, err := tx.Exec(ctx, `
		insert into notif_message (message_id, event_id, party_id, channel, subject, body, status, last_error)
		values ($1, $2, $3, $4, $5, $6, $7, $8)
		on conflict (event_id, channel) do nothing`,
		messageID, eventID, m.PartyID, m.Channel, m.Subject, m.Body, status, lastErr)
	if err != nil {
		return fmt.Errorf("store: enqueue: %w", err)
	}
	return nil
}

// Claimed is a pending message taken for sending.
type Claimed struct {
	MessageID string
	PartyID   string
	Channel   string
	Subject   string
	Body      string
	Attempts  int
}

// Claim takes up to limit pending messages that have attempts left. Rows are locked with SKIP LOCKED, so several senders
// can run at once. Each claim counts as an attempt.
func Claim(ctx context.Context, pool *pgxpool.Pool, limit, maxAttempts int) ([]Claimed, error) {
	rows, err := pool.Query(ctx, `
		update notif_message set attempts = attempts + 1
		where message_id in (
		  select message_id from notif_message
		  where status = 'PENDING' and attempts < $2
		  order by created_at for update skip locked limit $1)
		returning message_id, party_id, channel, subject, body, attempts`, limit, maxAttempts)
	if err != nil {
		return nil, fmt.Errorf("store: claim: %w", err)
	}
	defer rows.Close()
	var out []Claimed
	for rows.Next() {
		var c Claimed
		if err := rows.Scan(&c.MessageID, &c.PartyID, &c.Channel, &c.Subject, &c.Body, &c.Attempts); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// MarkSent records a delivered message.
func MarkSent(ctx context.Context, pool *pgxpool.Pool, messageID string) error {
	_, err := pool.Exec(ctx, `update notif_message set status = 'SENT', sent_at = now(), last_error = '' where message_id = $1`, messageID)
	return err
}

// MarkFailed records a failed attempt. When the attempts are used up, the message is FAILED and stays visible.
func MarkFailed(ctx context.Context, pool *pgxpool.Pool, messageID string, maxAttempts int, cause string) error {
	_, err := pool.Exec(ctx, `
		update notif_message set
		  status = case when attempts >= $2 then 'FAILED' else 'PENDING' end,
		  last_error = $3
		where message_id = $1`, messageID, maxAttempts, cause)
	return err
}

// Status returns the status of messages for an event, for tests and operators.
func Status(ctx context.Context, pool *pgxpool.Pool, eventID string) (map[string]string, error) {
	rows, err := pool.Query(ctx, `select channel, status from notif_message where event_id = $1`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var ch, st string
		if err := rows.Scan(&ch, &st); err != nil {
			return nil, err
		}
		out[ch] = st
	}
	return out, rows.Err()
}
