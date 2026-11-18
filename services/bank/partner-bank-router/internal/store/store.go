// Package store keeps partner outcomes (for health) and payment pins (so a payment stays with the partner it went to).
package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/khayaai/forge-mutual/services/bank/partner-bank-router/internal/router"
)

const schema = `
create table if not exists pbr_outcome (
  seq      bigserial   primary key,
  partner  text        not null,
  asset    text        not null,
  ok       boolean     not null,
  at       timestamptz not null default now()
);
create index if not exists pbr_outcome_partner on pbr_outcome (partner, seq desc);

-- A payment is pinned to the partner it was first sent to, before anything is sent. A retry or a status check goes to
-- the same partner, never to another one that might pay the same money again.
create table if not exists pbr_route (
  payment_id  text        primary key,
  partner     text        not null,
  asset       text        not null,
  created_at  timestamptz not null default now()
);
`

// ErrNotPinned is returned for a payment the router has never sent.
var ErrNotPinned = errors.New("store: payment has no partner")

// Open applies the schema. It is idempotent.
func Open(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, schema); err != nil {
		return fmt.Errorf("store: schema: %w", err)
	}
	return nil
}

// Record stores one outcome for a partner.
func Record(ctx context.Context, pool *pgxpool.Pool, partner, asset string, ok bool) error {
	_, err := pool.Exec(ctx, `insert into pbr_outcome (partner, asset, ok) values ($1, $2, $3)`, partner, asset, ok)
	return err
}

// Window returns the last n outcomes for a partner, oldest first.
func Window(ctx context.Context, pool *pgxpool.Pool, partner string, n int) (router.Window, error) {
	rows, err := pool.Query(ctx, `
		select ok from (select ok, seq from pbr_outcome where partner = $1 order by seq desc limit $2) recent
		order by seq`, partner, n)
	if err != nil {
		return router.Window{}, err
	}
	defer rows.Close()
	var w router.Window
	for rows.Next() {
		var ok bool
		if err := rows.Scan(&ok); err != nil {
			return router.Window{}, err
		}
		w.Outcomes = append(w.Outcomes, ok)
	}
	return w, rows.Err()
}

// Pin records the partner for a payment, unless one is already recorded. It returns the partner that holds the payment.
func Pin(ctx context.Context, pool *pgxpool.Pool, paymentID, partner, asset string) (string, error) {
	if _, err := pool.Exec(ctx, `
		insert into pbr_route (payment_id, partner, asset) values ($1, $2, $3)
		on conflict (payment_id) do nothing`, paymentID, partner, asset); err != nil {
		return "", err
	}
	var held string
	err := pool.QueryRow(ctx, `select partner from pbr_route where payment_id = $1`, paymentID).Scan(&held)
	return held, err
}

// PinnedPartner returns the partner that holds a payment.
func PinnedPartner(ctx context.Context, pool *pgxpool.Pool, paymentID string) (string, error) {
	var partner string
	err := pool.QueryRow(ctx, `select partner from pbr_route where payment_id = $1`, paymentID).Scan(&partner)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotPinned
	}
	return partner, err
}
