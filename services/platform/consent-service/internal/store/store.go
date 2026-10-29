// Package store is the consent database.
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

// ErrNotFound is returned for an unknown consent.
var ErrNotFound = errors.New("store: not found")

// ErrDuplicate is returned when a unique constraint is violated.
var ErrDuplicate = errors.New("store: duplicate")

// Consent is one grant.
type Consent struct {
	ID          string
	PartyID     string
	Purpose     string
	Scopes      []string
	DataSources []string
	GrantedAt   time.Time
	ExpiresAt   *time.Time
	RevokedAt   *time.Time
	EvidenceRef string
	GrantedBy   string
}

// Store wraps the pool.
type Store struct{ pool *pgxpool.Pool }

// Querier is satisfied by *pgxpool.Pool and pgx.Tx.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
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

// Pool exposes the pool.
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

const consentColumns = `consent_id::text, party_id::text, purpose, scopes, data_sources, granted_at,
	expires_at, revoked_at, evidence_ref, granted_by`

func scan(row pgx.Row) (Consent, error) {
	var c Consent
	err := row.Scan(&c.ID, &c.PartyID, &c.Purpose, &c.Scopes, &c.DataSources, &c.GrantedAt,
		&c.ExpiresAt, &c.RevokedAt, &c.EvidenceRef, &c.GrantedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return Consent{}, ErrNotFound
	}
	return c, err
}

// Insert writes a consent.
func Insert(ctx context.Context, q Querier, c Consent) error {
	_, err := q.Exec(ctx, `insert into consent (consent_id, party_id, purpose, scopes, data_sources, granted_at,
		expires_at, evidence_ref, granted_by) values ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9)`,
		c.ID, c.PartyID, c.Purpose, c.Scopes, c.DataSources, c.GrantedAt, c.ExpiresAt, c.EvidenceRef, c.GrantedBy)
	return translate(err)
}

// Get reads one consent.
func Get(ctx context.Context, q Querier, id string) (Consent, error) {
	return scan(q.QueryRow(ctx, `select `+consentColumns+` from consent where consent_id = $1::uuid`, id))
}

// Lock reads one consent and locks its row.
func Lock(ctx context.Context, q Querier, id string) (Consent, error) {
	return scan(q.QueryRow(ctx, `select `+consentColumns+` from consent where consent_id = $1::uuid for update`, id))
}

// Revoke sets revoked_at if the consent is not already revoked.
func Revoke(ctx context.Context, q Querier, id string) error {
	_, err := q.Exec(ctx, `update consent set revoked_at = now() where consent_id = $1::uuid and revoked_at is null`, id)
	return err
}

// ListForParty returns every consent of a party, revoked or not, oldest first.
func ListForParty(ctx context.Context, q Querier, partyID string) ([]Consent, error) {
	rows, err := q.Query(ctx, `select `+consentColumns+` from consent where party_id = $1::uuid order by granted_at, consent_id`, partyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Consent
	for rows.Next() {
		c, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// FindValid returns one consent that covers purpose and scope right now: not revoked, not expired.
func FindValid(ctx context.Context, q Querier, partyID, purpose, scope string) (Consent, error) {
	return scan(q.QueryRow(ctx, `select `+consentColumns+` from consent
		where party_id = $1::uuid and purpose = $2 and $3 = any(scopes)
		  and revoked_at is null and (expires_at is null or expires_at > now())
		order by granted_at limit 1`, partyID, purpose, scope))
}

// GetIdempotency returns the hash and resource recorded for a key.
func GetIdempotency(ctx context.Context, q Querier, scope, key string) ([]byte, string, bool, error) {
	var hash []byte
	var id string
	err := q.QueryRow(ctx, `select request_hash, resource_id::text from idempotency where scope = $1 and idem_key = $2`,
		scope, key).Scan(&hash, &id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", false, nil
	}
	if err != nil {
		return nil, "", false, err
	}
	return hash, id, true, nil
}

// PutIdempotency records a key against the consent it created.
func PutIdempotency(ctx context.Context, q Querier, scope, key string, hash []byte, resourceID string) error {
	_, err := q.Exec(ctx, `insert into idempotency (scope, idem_key, request_hash, resource_id) values ($1, $2, $3, $4::uuid)`,
		scope, key, hash, resourceID)
	return translate(err)
}

func translate(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		return ErrDuplicate
	}
	return err
}
