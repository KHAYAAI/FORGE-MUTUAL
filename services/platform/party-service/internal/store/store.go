// Package store is the party database. It owns the SQL; the service owns the rules.
package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	_ "embed"
	"github.com/khayaai/forge-mutual/pkg/outbox"
)

//go:embed schema.sql
var schema string

// Errors the service maps to API codes.
var (
	ErrNotFound  = errors.New("store: not found")
	ErrDuplicate = errors.New("store: duplicate")
	ErrStale     = errors.New("store: row changed concurrently")
)

// Party is a stored party. Enum fields hold protobuf enum names.
type Party struct {
	ID                 string
	Type               string
	Status             string
	LegalName          string
	RegistrationNumber string
	SealedNationalID   []byte
	NationalIDIndex    []byte
	NationalIDMasked   string
	Country            string
	Tier               string
	CreatedAt          time.Time
}

// Relationship links two parties.
type Relationship struct {
	ID     string
	From   string
	To     string
	Type   string
	Bps    uint32
	Active bool
}

// Store wraps the connection pool.
type Store struct {
	pool *pgxpool.Pool
}

// Querier is satisfied by both *pgxpool.Pool and pgx.Tx.
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

// translate maps Postgres constraint errors to store errors.
func translate(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		return ErrDuplicate
	}
	return err
}

const partyColumns = `party_id::text, party_type, status, legal_name, coalesce(registration_number, ''),
	national_id_sealed, national_id_index, coalesce(national_id_masked, ''), country, tier, created_at`

func scanParty(row pgx.Row) (Party, error) {
	var p Party
	err := row.Scan(&p.ID, &p.Type, &p.Status, &p.LegalName, &p.RegistrationNumber,
		&p.SealedNationalID, &p.NationalIDIndex, &p.NationalIDMasked, &p.Country, &p.Tier, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Party{}, ErrNotFound
	}
	return p, err
}

// InsertParty writes a new party.
func InsertParty(ctx context.Context, q Querier, p Party) error {
	_, err := q.Exec(ctx, `
		insert into party (party_id, party_type, status, legal_name, registration_number,
			national_id_sealed, national_id_index, national_id_masked, country, tier)
		values ($1::uuid, $2, $3, $4, nullif($5, ''), $6, $7, nullif($8, ''), $9, $10)`,
		p.ID, p.Type, p.Status, p.LegalName, p.RegistrationNumber,
		p.SealedNationalID, p.NationalIDIndex, p.NationalIDMasked, p.Country, p.Tier)
	return translate(err)
}

// GetParty reads one party.
func GetParty(ctx context.Context, q Querier, id string) (Party, error) {
	return scanParty(q.QueryRow(ctx, `select `+partyColumns+` from party where party_id = $1::uuid`, id))
}

// LockParty reads one party and locks its row for the rest of the transaction.
func LockParty(ctx context.Context, q Querier, id string) (Party, error) {
	return scanParty(q.QueryRow(ctx, `select `+partyColumns+` from party where party_id = $1::uuid for update`, id))
}

// SetStatus changes status only if it still equals from. A concurrent change yields ErrStale.
func SetStatus(ctx context.Context, q Querier, id, from, to string) error {
	tag, err := q.Exec(ctx, `update party set status = $3, updated_at = now()
		where party_id = $1::uuid and status = $2`, id, from, to)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrStale
	}
	return nil
}

// SetTier sets the tier label used for limits.
func SetTier(ctx context.Context, q Querier, id, tier string) error {
	_, err := q.Exec(ctx, `update party set tier = $2, updated_at = now() where party_id = $1::uuid`, id, tier)
	return err
}

// InsertRelationship writes a relationship. An active duplicate yields ErrDuplicate.
func InsertRelationship(ctx context.Context, q Querier, r Relationship) error {
	_, err := q.Exec(ctx, `insert into relationship (relationship_id, from_party_id, to_party_id, rel_type, percentage_bps, active)
		values ($1::uuid, $2::uuid, $3::uuid, $4, $5, true)`, r.ID, r.From, r.To, r.Type, r.Bps)
	return translate(err)
}

// ListRelationships returns relationships in which the party is either end.
func ListRelationships(ctx context.Context, q Querier, partyID string) ([]Relationship, error) {
	rows, err := q.Query(ctx, `select relationship_id::text, from_party_id::text, to_party_id::text, rel_type,
		percentage_bps, active from relationship
		where from_party_id = $1::uuid or to_party_id = $1::uuid order by relationship_id`, partyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Relationship
	for rows.Next() {
		var r Relationship
		if err := rows.Scan(&r.ID, &r.From, &r.To, &r.Type, &r.Bps, &r.Active); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetIdempotency returns the hash and resource recorded for a key, if any.
func GetIdempotency(ctx context.Context, q Querier, scope, key string) (hash []byte, resourceID string, found bool, err error) {
	err = q.QueryRow(ctx, `select request_hash, resource_id::text from idempotency where scope = $1 and idem_key = $2`,
		scope, key).Scan(&hash, &resourceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, "", false, nil
	}
	if err != nil {
		return nil, "", false, err
	}
	return hash, resourceID, true, nil
}

// PutIdempotency records a key against the resource it created.
func PutIdempotency(ctx context.Context, q Querier, scope, key string, hash []byte, resourceID string) error {
	_, err := q.Exec(ctx, `insert into idempotency (scope, idem_key, request_hash, resource_id) values ($1, $2, $3, $4::uuid)`,
		scope, key, hash, resourceID)
	return translate(err)
}

// GetRelationship reads one relationship by ID, used to replay an idempotent add.
func GetRelationship(ctx context.Context, q Querier, id string) (Relationship, error) {
	var r Relationship
	err := q.QueryRow(ctx, `select relationship_id::text, from_party_id::text, to_party_id::text, rel_type,
		percentage_bps, active from relationship where relationship_id = $1::uuid`, id).
		Scan(&r.ID, &r.From, &r.To, &r.Type, &r.Bps, &r.Active)
	if errors.Is(err, pgx.ErrNoRows) {
		return Relationship{}, ErrNotFound
	}
	return r, err
}
