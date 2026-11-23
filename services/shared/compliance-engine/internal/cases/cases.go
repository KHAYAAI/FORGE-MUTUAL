// Package cases holds compliance cases: what the monitoring rules raised, with an SLA and a maker-checker close.
package cases

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/khayaai/forge-mutual/pkg/uuid"
	"github.com/khayaai/forge-mutual/services/shared/compliance-engine/internal/monitor"
)

//go:embed schema.sql
var schema string

// SLA is how long a case may stay open, by severity. Placeholders (VERIFY): counsel and the compliance officer set them.
var SLA = map[string]time.Duration{
	"CRITICAL": 24 * time.Hour,
	"WARNING":  72 * time.Hour,
}

// Errors.
var (
	ErrNotFound      = errors.New("cases: not found")
	ErrNotOpen       = errors.New("cases: case is not open")
	ErrSameClerk     = errors.New("cases: the person who opened a case cannot close it")
	ErrNoClosingNote = errors.New("cases: a closing note of at least 20 characters is required")
)

// Case is one compliance case.
type Case struct {
	CaseID   string
	PartyID  string
	Rule     string
	Severity string
	Detail   string
	Evidence []string
	Status   string
	OpenedBy string
	OpenedAt time.Time
	SLADueAt time.Time
	ClosedBy string
	ClosedAt *time.Time
	Closing  string
}

// Store holds cases.
type Store struct {
	pool *pgxpool.Pool
}

// Open applies the schema. It is idempotent.
func Open(ctx context.Context, pool *pgxpool.Pool) error {
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `select pg_advisory_xact_lock(7743)`); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, schema)
		return err
	})
}

// New returns a store over pool.
func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// OpenFromFindings opens a case for each finding that has no open case for the same party and rule. It returns how
// many cases it opened. A repeat finding for a party already under review does not open a second case.
func (s *Store) OpenFromFindings(ctx context.Context, findings []monitor.Finding, actor string, now time.Time) (int, error) {
	return openFindings(ctx, s.pool, findings, actor, now)
}

// OpenFromFindingsTx is OpenFromFindings inside the caller's transaction, so a case commits with the event that raised it.
func OpenFromFindingsTx(ctx context.Context, tx pgx.Tx, findings []monitor.Finding, actor string, now time.Time) (int, error) {
	return openFindings(ctx, tx, findings, actor, now)
}

// execer is what both a pool and a transaction can run.
type execer interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

func openFindings(ctx context.Context, ex execer, findings []monitor.Finding, actor string, now time.Time) (int, error) {
	opened := 0
	for _, f := range findings {
		sla, ok := SLA[f.Severity]
		if !ok {
			return opened, fmt.Errorf("cases: no SLA for severity %q", f.Severity)
		}
		id, err := uuid.NewV4()
		if err != nil {
			return opened, err
		}
		tag, err := ex.Exec(ctx, `
			insert into compliance_case (case_id, party_id, rule, severity, detail, evidence, status, opened_by, opened_at, sla_due_at)
			values ($1, $2, $3, $4, $5, $6, 'OPEN', $7, $8, $9)
			on conflict (party_id, rule) where status = 'OPEN' do nothing`,
			id, f.PartyID, f.Rule, f.Severity, f.Detail, f.Evidence, actor, now.UTC(), now.UTC().Add(sla))
		if err != nil {
			return opened, fmt.Errorf("cases: open: %w", err)
		}
		opened += int(tag.RowsAffected())
	}
	return opened, nil
}

// Close closes an open case. The closer must not be the person who opened it, and must leave a real note.
func (s *Store) Close(ctx context.Context, caseID, actor, note string, now time.Time) error {
	if len(note) < 20 {
		return ErrNoClosingNote
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var status, openedBy string
		err := tx.QueryRow(ctx, `select status, opened_by from compliance_case where case_id = $1 for update`, caseID).Scan(&status, &openedBy)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if status != "OPEN" {
			return ErrNotOpen
		}
		if actor == openedBy {
			return ErrSameClerk
		}
		_, err = tx.Exec(ctx, `
			update compliance_case set status = 'CLOSED', closed_by = $2, closed_at = $3, closing_note = $4
			where case_id = $1`, caseID, actor, now.UTC(), note)
		return err
	})
}

// Overdue lists open cases past their SLA at now, oldest first.
func (s *Store) Overdue(ctx context.Context, now time.Time) ([]Case, error) {
	rows, err := s.pool.Query(ctx, `
		select case_id, party_id, rule, severity, detail, evidence, status, opened_by, opened_at, sla_due_at
		from compliance_case where status = 'OPEN' and sla_due_at < $1 order by sla_due_at`, now.UTC())
	if err != nil {
		return nil, fmt.Errorf("cases: overdue: %w", err)
	}
	defer rows.Close()
	var out []Case
	for rows.Next() {
		var c Case
		if err := rows.Scan(&c.CaseID, &c.PartyID, &c.Rule, &c.Severity, &c.Detail, &c.Evidence, &c.Status, &c.OpenedBy, &c.OpenedAt, &c.SLADueAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Get returns one case.
func (s *Store) Get(ctx context.Context, caseID string) (Case, error) {
	var c Case
	err := s.pool.QueryRow(ctx, `
		select case_id, party_id, rule, severity, detail, evidence, status, opened_by, opened_at, sla_due_at,
		       coalesce(closed_by, ''), closed_at, coalesce(closing_note, '')
		from compliance_case where case_id = $1`, caseID).
		Scan(&c.CaseID, &c.PartyID, &c.Rule, &c.Severity, &c.Detail, &c.Evidence, &c.Status, &c.OpenedBy, &c.OpenedAt, &c.SLADueAt,
			&c.ClosedBy, &c.ClosedAt, &c.Closing)
	if errors.Is(err, pgx.ErrNoRows) {
		return Case{}, ErrNotFound
	}
	return c, err
}

// List returns cases, oldest SLA first. With openOnly, only open cases are returned. Limit bounds the result.
func (s *Store) List(ctx context.Context, openOnly bool, limit int) ([]Case, error) {
	q := `select case_id, party_id, rule, severity, detail, evidence, status, opened_by, opened_at, sla_due_at,
		coalesce(closed_by, ''), closed_at, coalesce(closing_note, '')
		from compliance_case`
	if openOnly {
		q += ` where status = 'OPEN'`
	}
	q += ` order by sla_due_at, case_id limit $1`
	rows, err := s.pool.Query(ctx, q, limit)
	if err != nil {
		return nil, fmt.Errorf("cases: list: %w", err)
	}
	defer rows.Close()
	var out []Case
	for rows.Next() {
		var c Case
		if err := rows.Scan(&c.CaseID, &c.PartyID, &c.Rule, &c.Severity, &c.Detail, &c.Evidence, &c.Status, &c.OpenedBy,
			&c.OpenedAt, &c.SLADueAt, &c.ClosedBy, &c.ClosedAt, &c.Closing); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
