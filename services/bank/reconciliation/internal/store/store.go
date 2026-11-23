// Package store persists reconciliation runs, breaks and investigation notes in Postgres.
package store

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/khayaai/forge-mutual/pkg/uuid"
	"github.com/khayaai/forge-mutual/services/bank/reconciliation/internal/recon"
)

//go:embed schema.sql
var schema string

// Outcomes of a run.
const (
	OutcomeConclusive   = "CONCLUSIVE"
	OutcomeInconclusive = "INCONCLUSIVE"
)

// Break statuses.
const (
	StatusOpen    = "OPEN"
	StatusCleared = "CLEARED"
)

// newID returns a random UUID. A failure of crypto/rand is unrecoverable, so it is not returned.
func newID() string {
	id, err := uuid.NewV4()
	if err != nil {
		panic(fmt.Sprintf("store: random id: %v", err))
	}
	return id
}

// ErrNotFound is returned when a break does not exist.
var ErrNotFound = errors.New("store: not found")

// Run is one reconciliation run for one asset.
type Run struct {
	RunID      string
	Asset      string
	StartedAt  time.Time
	FinishedAt time.Time
	Outcome    string
	Checks     []recon.Result
}

// AllMatched reports whether the run concluded and every check matched.
func (r Run) AllMatched() bool {
	return r.Outcome == OutcomeConclusive && recon.AllMatched(r.Checks)
}

// Break is a mismatch on one check for one asset.
type Break struct {
	BreakID           string
	Asset             string
	Check             string
	DifferenceMinor   int64
	Detail            string
	Status            string
	OpenedAt          time.Time
	LastSeenAt        time.Time
	ClearedAt         *time.Time
	InvestigationNote string
	InvestigatedBy    string
	InvestigatedAt    *time.Time
}

// Store is the reconciliation database.
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
	if _, err := pool.Exec(ctx, schema); err != nil {
		pool.Close()
		return nil, fmt.Errorf("store: schema: %w", err)
	}
	return &Store{pool: pool, now: time.Now}, nil
}

// Close releases the pool.
func (s *Store) Close() { s.pool.Close() }

// Pool exposes the pool to tests in this module.
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

// SetClock replaces the clock. Tests use it; production never does.
func (s *Store) SetClock(now func() time.Time) { s.now = now }

// Commit writes a run. A conclusive run also opens, updates or clears breaks, in the same transaction, so the gate can
// never see a run without its breaks. An inconclusive run changes no break.
func (s *Store) Commit(ctx context.Context, run Run) error {
	if run.RunID == "" {
		run.RunID = newID()
	}
	failed := []string{}
	for _, c := range run.Checks {
		if !c.Matched {
			failed = append(failed, c.Check)
		}
	}
	checks := encodeChecks(run.Checks)
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			insert into recon_run (run_id, asset, started_at, finished_at, outcome, all_matched, failed_checks, checks)
			values ($1, $2, $3, $4, $5, $6, $7, $8)`,
			run.RunID, run.Asset, run.StartedAt.UTC(), run.FinishedAt.UTC(), run.Outcome,
			run.AllMatched(), failed, checks); err != nil {
			return fmt.Errorf("store: insert run: %w", err)
		}
		if run.Outcome != OutcomeConclusive {
			return nil
		}
		for _, c := range run.Checks {
			if c.Matched {
				if _, err := tx.Exec(ctx, `
					update recon_break set status = 'CLEARED', cleared_at = $3
					where asset = $1 and check_name = $2 and status = 'OPEN'`,
					run.Asset, c.Check, run.FinishedAt.UTC()); err != nil {
					return fmt.Errorf("store: clear break: %w", err)
				}
				continue
			}
			if _, err := tx.Exec(ctx, `
				insert into recon_break (break_id, asset, check_name, difference_minor, detail, status, opened_at, last_seen_at)
				values ($1, $2, $3, $4, $5, 'OPEN', $6, $6)
				on conflict (asset, check_name) where status = 'OPEN'
				do update set difference_minor = excluded.difference_minor, detail = excluded.detail,
				              last_seen_at = excluded.last_seen_at`,
				newID(), run.Asset, c.Check, fmt.Sprint(c.Difference), c.Detail, run.FinishedAt.UTC()); err != nil {
				return fmt.Errorf("store: open break: %w", err)
			}
		}
		return nil
	})
}

// GateState is what the gate reads: the latest conclusive run per asset, and any open breaks.
type GateState struct {
	Open    bool
	Reasons []string
}

// Gate reports whether outbound payments may proceed. It is open only when every asset has a conclusive run no
// older than maxAge in which every check matched, and no break is open. It fails closed.
func (s *Store) Gate(ctx context.Context, assets []string, now time.Time, maxAge time.Duration) (GateState, error) {
	var reasons []string
	for _, asset := range assets {
		var finished time.Time
		var matched bool
		var failed []string
		err := s.pool.QueryRow(ctx, `
			select finished_at, all_matched, failed_checks from recon_run
			where asset = $1 and outcome = 'CONCLUSIVE'
			order by finished_at desc limit 1`, asset).Scan(&finished, &matched, &failed)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			reasons = append(reasons, asset+": no conclusive reconciliation run yet")
			continue
		case err != nil:
			return GateState{}, fmt.Errorf("store: gate: %w", err)
		}
		if age := now.Sub(finished); age > maxAge {
			reasons = append(reasons, fmt.Sprintf("%s: last conclusive run is %s old, limit %s", asset, age.Round(time.Second), maxAge))
		}
		if !matched {
			reasons = append(reasons, fmt.Sprintf("%s: latest run has breaks on %v", asset, failed))
		}
	}
	var open int
	if err := s.pool.QueryRow(ctx, `select count(*) from recon_break where status = 'OPEN'`).Scan(&open); err != nil {
		return GateState{}, fmt.Errorf("store: count breaks: %w", err)
	}
	if open > 0 {
		reasons = append(reasons, fmt.Sprintf("%d break(s) open", open))
	}
	return GateState{Open: len(reasons) == 0, Reasons: reasons}, nil
}

// ListBreaks returns breaks, newest first.
func (s *Store) ListBreaks(ctx context.Context, openOnly bool) ([]Break, error) {
	q := `select break_id, asset, check_name, difference_minor, detail, status, opened_at, last_seen_at, cleared_at,
	             investigation_note, investigated_by, investigated_at
	      from recon_break`
	if openOnly {
		q += ` where status = 'OPEN'`
	}
	q += ` order by opened_at desc, break_id`
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("store: list breaks: %w", err)
	}
	defer rows.Close()
	var out []Break
	for rows.Next() {
		b, err := scanBreak(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// Investigate records a human's findings. It never changes the gate: only a later conclusive run can clear a break.
func (s *Store) Investigate(ctx context.Context, breakID, actor, note string) (Break, error) {
	if actor == "" || note == "" {
		return Break{}, errors.New("store: actor and note are required")
	}
	now := s.now().UTC()
	var out Break
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			update recon_break set investigation_note = $2, investigated_by = $3, investigated_at = $4
			where break_id = $1
			returning break_id, asset, check_name, difference_minor, detail, status, opened_at, last_seen_at, cleared_at,
			          investigation_note, investigated_by, investigated_at`,
			breakID, note, actor, now)
		b, err := scanBreak(row)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		out = b
		_, err = tx.Exec(ctx, `insert into recon_investigation (break_id, actor, note, created_at) values ($1, $2, $3, $4)`,
			breakID, actor, note, now)
		return err
	})
	return out, err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanBreak(r scanner) (Break, error) {
	var (
		b    Break
		diff string
	)
	if err := r.Scan(&b.BreakID, &b.Asset, &b.Check, &diff, &b.Detail, &b.Status, &b.OpenedAt, &b.LastSeenAt,
		&b.ClearedAt, &b.InvestigationNote, &b.InvestigatedBy, &b.InvestigatedAt); err != nil {
		return Break{}, err
	}
	if _, err := fmt.Sscan(diff, &b.DifferenceMinor); err != nil {
		return Break{}, fmt.Errorf("store: difference %q: %w", diff, err)
	}
	return b, nil
}
