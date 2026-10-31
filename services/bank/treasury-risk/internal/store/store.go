// Package store keeps treasury snapshots and alerts, and writes an outbox event on each alert transition.
package store

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/proto"

	eventsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/events/v1"
	"github.com/khayaai/forge-mutual/pkg/outbox"
	"github.com/khayaai/forge-mutual/pkg/uuid"
	"github.com/khayaai/forge-mutual/services/bank/treasury-risk/internal/risk"
)

//go:embed schema.sql
var schema string

// TopicAlert is the outbox topic for alert transitions (ARCHITECTURE.md Appendix C).
const TopicAlert = "forge.bank.risk.alert.v1"

// Store is the treasury database.
type Store struct {
	pool *pgxpool.Pool
}

// Open applies the schema and the outbox table. It is idempotent.
func Open(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("store: connect: %w", err)
	}
	if err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `select pg_advisory_xact_lock(7742)`); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, schema)
		return err
	}); err != nil {
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

// Pool exposes the pool for health checks and tests.
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

// Previous returns the latest snapshot for asset taken at or before t, or nil if there is none.
func (s *Store) Previous(ctx context.Context, asset string, t time.Time) (*risk.Snapshot, error) {
	var sn risk.Snapshot
	err := s.pool.QueryRow(ctx, `
		select asset, at, deposits, largest, largest_account, pool, suspense from treasury_snapshot
		where asset = $1 and at <= $2 order by at desc limit 1`, asset, t.UTC()).
		Scan(&sn.Asset, &sn.At, &sn.Deposits, &sn.LargestDeposit, &sn.LargestAccount, &sn.Pool, &sn.Suspense)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("store: previous: %w", err)
	}
	return &sn, nil
}

// Commit records a snapshot and reconciles the open alerts with the alerts that hold now, in one transaction. An alert
// that newly holds is raised and emits an event. One that no longer holds is resolved and emits an event. One that
// still holds is updated quietly.
func (s *Store) Commit(ctx context.Context, cur risk.Snapshot, holding []risk.Alert) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			insert into treasury_snapshot (asset, at, deposits, largest, largest_account, pool, suspense)
			values ($1, $2, $3, $4, $5, $6, $7)`,
			cur.Asset, cur.At.UTC(), cur.Deposits, cur.LargestDeposit, cur.LargestAccount, cur.Pool, cur.Suspense); err != nil {
			return fmt.Errorf("store: snapshot: %w", err)
		}

		open := map[string]string{} // kind to alert ID
		rows, err := tx.Query(ctx, `select kind, alert_id from treasury_alert where asset = $1 and state = 'RAISED'`, cur.Asset)
		if err != nil {
			return fmt.Errorf("store: open alerts: %w", err)
		}
		for rows.Next() {
			var kind, id string
			if err := rows.Scan(&kind, &id); err != nil {
				rows.Close()
				return err
			}
			open[kind] = id
		}
		rows.Close()

		seen := map[string]bool{}
		now := cur.At.UTC()
		for _, a := range holding {
			seen[a.Kind] = true
			if id, ok := open[a.Kind]; ok {
				if _, err := tx.Exec(ctx, `
					update treasury_alert set detail = $2, severity = $3, metric_bps = $4, threshold_bps = $5
					where alert_id = $1`, id, a.Detail, a.Severity, a.MetricBps, a.ThresholdBps); err != nil {
					return fmt.Errorf("store: update alert: %w", err)
				}
				continue
			}
			id := newID()
			if _, err := tx.Exec(ctx, `
				insert into treasury_alert (alert_id, kind, asset, severity, state, detail, metric_bps, threshold_bps, raised_at)
				values ($1, $2, $3, $4, 'RAISED', $5, $6, $7, $8)`,
				id, a.Kind, a.Asset, a.Severity, a.Detail, a.MetricBps, a.ThresholdBps, now); err != nil {
				return fmt.Errorf("store: raise alert: %w", err)
			}
			if err := emit(ctx, tx, id, a, "RAISED", now); err != nil {
				return err
			}
		}

		for kind, id := range open {
			if seen[kind] {
				continue
			}
			if _, err := tx.Exec(ctx, `
				update treasury_alert set state = 'RESOLVED', resolved_at = $2 where alert_id = $1`, id, now); err != nil {
				return fmt.Errorf("store: resolve alert: %w", err)
			}
			if err := emit(ctx, tx, id, risk.Alert{Kind: kind, Asset: cur.Asset, Severity: "", Detail: "cleared"}, "RESOLVED", now); err != nil {
				return err
			}
		}
		return nil
	})
}

// RaisedAlerts lists alerts that currently hold, for review.
func (s *Store) RaisedAlerts(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx, `select kind || ' ' || asset from treasury_alert where state = 'RAISED' order by kind, asset`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func emit(ctx context.Context, tx pgx.Tx, id string, a risk.Alert, state string, at time.Time) error {
	payload, err := proto.Marshal(&eventsv1.RiskAlertRaised{
		AlertId: id, Kind: a.Kind, Asset: a.Asset, Severity: a.Severity, State: state, Detail: a.Detail,
		MetricBps: fmt.Sprint(a.MetricBps), ThresholdBps: fmt.Sprint(a.ThresholdBps),
		ObservedAt: at.UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return err
	}
	return outbox.Append(ctx, tx, outbox.Event{EventID: newID(), Topic: TopicAlert, Key: []byte(id), Payload: payload})
}

// newID returns a random UUID. A failure of crypto/rand is unrecoverable.
func newID() string {
	id, err := uuid.NewV4()
	if err != nil {
		panic(fmt.Sprintf("store: random id: %v", err))
	}
	return id
}
