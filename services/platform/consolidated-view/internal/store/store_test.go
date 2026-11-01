package store

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// openTest needs CONSOLIDATED_TEST_DSN, a throwaway database. Tables are truncated.
func openTest(t *testing.T) (*Store, *pgxpool.Pool) {
	t.Helper()
	dsn := os.Getenv("CONSOLIDATED_TEST_DSN")
	if dsn == "" {
		t.Skip("CONSOLIDATED_TEST_DSN not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := Open(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `truncate cv_activity, cv_balance, cv_account, cv_recon_run`); err != nil {
		t.Fatal(err)
	}
	return New(pool), pool
}

func apply(t *testing.T, pool *pgxpool.Pool, tr Transfer) {
	t.Helper()
	if err := pgx.BeginFunc(context.Background(), pool, func(tx pgx.Tx) error { return Project(context.Background(), tx, tr) }); err != nil {
		t.Fatal(err)
	}
}

var t0 = time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)

// The same transfer applied twice counts once: the activity row is unique per account and direction.
func TestProjectionCountsEachTransferOnce(t *testing.T) {
	_, pool := openTest(t)
	tr := Transfer{ID: "t1", Asset: "ZAR", Debit: "A", Credit: "B", Amount: 500, Code: 10, At: t0}
	apply(t, pool, tr)
	apply(t, pool, tr)
	var a, b int64
	_ = pool.QueryRow(context.Background(), `select credit_minor - debit_minor from cv_balance where account_id = 'A'`).Scan(&a)
	_ = pool.QueryRow(context.Background(), `select credit_minor - debit_minor from cv_balance where account_id = 'B'`).Scan(&b)
	if a != -500 || b != 500 {
		t.Fatalf("balances after a replay: A=%d B=%d, want -500 and 500", a, b)
	}
}

// A pending transfer is a hold: it changes nothing until it is posted.
func TestPendingTransferIsNotPosted(t *testing.T) {
	s, pool := openTest(t)
	apply(t, pool, Transfer{ID: "hold", Asset: "ZAR", Debit: "A", Credit: "B", Amount: 900, Code: 11, Pending: true, At: t0})
	var n int
	_ = pool.QueryRow(context.Background(), `select count(*) from cv_activity`).Scan(&n)
	if n != 0 {
		t.Fatalf("a hold created %d activity rows", n)
	}
	_ = s
}

// Balances and activity are visible only to the owner. Anyone else gets the same answer as for a missing account.
func TestOwnershipIsEnforced(t *testing.T) {
	s, pool := openTest(t)
	ctx := context.Background()
	apply(t, pool, Transfer{ID: "t1", Asset: "ZAR", Debit: "A", Credit: "B", Amount: 700, Code: 10, At: t0})
	if _, err := pool.Exec(ctx, `update cv_account set owner_party_id = 'party-1', insured = true, product_id = 'business-account' where account_id = 'B'`); err != nil {
		t.Fatal(err)
	}

	bal, err := s.Balances(ctx, "party-1")
	if err != nil || len(bal) != 1 || bal[0].Available != 700 || !bal[0].Insured {
		t.Fatalf("owner balances = %+v, %v", bal, err)
	}
	if other, _ := s.Balances(ctx, "party-2"); len(other) != 0 {
		t.Fatalf("another party sees %d accounts", len(other))
	}
	if _, err := s.Activity(ctx, "party-2", "B", 0, 10); !errors.Is(err, ErrNotFound) {
		t.Fatalf("another party reading activity: %v, want ErrNotFound", err)
	}
	rows, err := s.Activity(ctx, "party-1", "B", 0, 10)
	if err != nil || len(rows) != 1 || rows[0].Direction != "IN" || rows[0].Amount != 700 {
		t.Fatalf("owner activity = %+v, %v", rows, err)
	}
}

// An account no reconciliation has claimed is invisible to every party. Failing closed is the safe default.
func TestUnclaimedAccountIsInvisible(t *testing.T) {
	s, pool := openTest(t)
	apply(t, pool, Transfer{ID: "t1", Asset: "ZAR", Debit: "A", Credit: "B", Amount: 1, Code: 10, At: t0})
	if got, _ := s.Balances(context.Background(), "anyone"); len(got) != 0 {
		t.Fatalf("an account without an owner was visible")
	}
}
