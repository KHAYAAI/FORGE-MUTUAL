package poster

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/khayaai/forge-mutual/services/bank/core-bridge/internal/chart"
	"github.com/khayaai/forge-mutual/services/bank/core-bridge/internal/finfake"
	"github.com/khayaai/forge-mutual/services/bank/core-bridge/internal/store"
)

// setup needs CORE_BRIDGE_TEST_DSN, a throwaway database. The bridge tables are truncated.
func setup(t *testing.T) (*pgxpool.Pool, *finfake.Fake, *Poster) {
	t.Helper()
	dsn := os.Getenv("CORE_BRIDGE_TEST_DSN")
	if dsn == "" {
		t.Skip("CORE_BRIDGE_TEST_DSN not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := store.Open(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `truncate cb_journal, cb_recon_run, cb_account`); err != nil {
		t.Fatal(err)
	}
	fake := finfake.New()
	client := fake.Serve(t)
	gl := map[string]int64{}
	for _, asset := range []string{"ZAR"} {
		for _, e := range []struct {
			code uint16
			kind int
		}{{1001, 2}, {3003, 1}} {
			id, err := client.CreateGLAccount(ctx, chart.GLCode(e.code, asset), "test", e.kind)
			if err != nil {
				t.Fatal(err)
			}
			gl[chart.GLCode(e.code, asset)] = id
		}
	}
	return pool, fake, &Poster{Pool: pool, Fin: client, OfficeID: 1, GL: gl, MaxAttempts: 5, Batch: 10}
}

func queue(t *testing.T, pool *pgxpool.Pool, id string) {
	t.Helper()
	err := pgx.BeginFunc(context.Background(), pool, func(tx pgx.Tx) error {
		return store.Queue(context.Background(), tx, id, "ZAR", 15000, 3003, 1001, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), store.StatusPending, "")
	})
	if err != nil {
		t.Fatal(err)
	}
}

// The normal path posts once and records Fineract's transaction id.
func TestPostsOnceAndRecordsTheTransaction(t *testing.T) {
	pool, fake, p := setup(t)
	queue(t, pool, "tr-1")
	posted, failed, err := p.Pass(context.Background())
	if err != nil || posted != 1 || failed != 0 {
		t.Fatalf("pass: posted=%d failed=%d err=%v", posted, failed, err)
	}
	status, txn, _, _ := store.Journal(context.Background(), pool, "tr-1")
	if status != store.StatusPosted || txn == "" || fake.Journals("tr-1") != 1 {
		t.Fatalf("status=%s txn=%s journals=%d", status, txn, fake.Journals("tr-1"))
	}
}

// The hard case. Fineract accepts the journal and then fails the response. The retry must find the journal by its
// reference and record it, not post it again. Two postings would be money counted twice in the general ledger.
func TestUncertainWriteIsNotRepeated(t *testing.T) {
	pool, fake, p := setup(t)
	ctx := context.Background()
	queue(t, pool, "tr-2")
	fake.AcceptThenFail = true

	if posted, failed, _ := p.Pass(ctx); posted != 0 || failed != 1 {
		t.Fatalf("first pass: posted=%d failed=%d; want the uncertain write to fail", posted, failed)
	}
	status, _, _, _ := store.Journal(ctx, pool, "tr-2")
	if status != store.StatusPending {
		t.Fatalf("an uncertain outcome must leave the journal pending, got %s", status)
	}
	if posted, _, err := p.Pass(ctx); err != nil || posted != 1 {
		t.Fatalf("retry: posted=%d err=%v", posted, err)
	}
	if n := fake.Journals("tr-2"); n != 1 {
		t.Fatalf("Fineract holds %d postings for one transfer; it must hold exactly 1", n)
	}
	status, txn, _, _ := store.Journal(ctx, pool, "tr-2")
	if status != store.StatusPosted || txn == "" {
		t.Fatalf("status = %s txn = %q", status, txn)
	}
}

// A refusal with a 4xx means nothing was written, so the journal fails at once with the reason kept.
func TestRefusalFailsAtOnce(t *testing.T) {
	pool, fake, p := setup(t)
	ctx := context.Background()
	queue(t, pool, "tr-3")
	fake.RefuseNext = true
	if _, failed, _ := p.Pass(ctx); failed != 1 {
		t.Fatalf("refusal: failed=%d", failed)
	}
	status, _, reason, _ := store.Journal(ctx, pool, "tr-3")
	if status != store.StatusFailed || reason == "" || fake.Journals("tr-3") != 0 {
		t.Fatalf("status=%s reason=%q journals=%d", status, reason, fake.Journals("tr-3"))
	}
}

// Only one poster runs at a time. A second poster that finds the lock held does nothing.
func TestSecondPosterDoesNothing(t *testing.T) {
	pool, _, p := setup(t)
	ctx := context.Background()
	queue(t, pool, "tr-4")
	held, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Release()
	if _, err := held.Exec(ctx, `select pg_advisory_lock($1)`, lockKey); err != nil {
		t.Fatal(err)
	}
	defer held.Exec(ctx, `select pg_advisory_unlock($1)`, lockKey) //nolint:errcheck
	if posted, failed, err := p.Pass(ctx); err != nil || posted != 0 || failed != 0 {
		t.Fatalf("a locked poster must do nothing: %d %d %v", posted, failed, err)
	}
	status, _, _, _ := store.Journal(ctx, pool, "tr-4")
	if status != store.StatusPending {
		t.Fatalf("status = %s", status)
	}
}
