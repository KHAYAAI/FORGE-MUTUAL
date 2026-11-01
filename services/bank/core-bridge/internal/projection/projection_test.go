package projection

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	eventsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/events/v1"
	"github.com/khayaai/forge-mutual/pkg/events"
	"github.com/khayaai/forge-mutual/services/bank/core-bridge/internal/store"
)

type fixed map[string]store.AccountInfo

func (f fixed) Resolve(_ context.Context, id string) (store.AccountInfo, error) {
	if a, ok := f[id]; ok {
		return a, nil
	}
	return store.AccountInfo{}, store.ErrUnknownAccount
}

func setup(t *testing.T) *pgxpool.Pool {
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
	if _, err := pool.Exec(ctx, `truncate cb_journal`); err != nil {
		t.Fatal(err)
	}
	return pool
}

func transfer(t *testing.T, id, debit, credit string, pending bool, code uint32) *eventsv1.EventEnvelope {
	t.Helper()
	_, data, err := events.Wrap(events.Params{Type: TopicTransferCommitted, Entity: "BANK", Producer: "ledger-gateway-bank"},
		&eventsv1.TransferCommitted{TransferId: id, DebitAccountId: debit, CreditAccountId: credit, Asset: "ZAR", AmountMinor: "15000", Code: code, Pending: pending})
	if err != nil {
		t.Fatal(err)
	}
	env, err := events.Open(data)
	if err != nil {
		t.Fatal(err)
	}
	return env
}

func apply(t *testing.T, pool *pgxpool.Pool, h func(context.Context, pgx.Tx) error) {
	t.Helper()
	if err := pgx.BeginFunc(context.Background(), pool, func(tx pgx.Tx) error { return h(context.Background(), tx) }); err != nil {
		t.Fatal(err)
	}
}

var res = fixed{
	"pool":  {Code: 3003, Asset: "ZAR"},
	"cust":  {Code: 1001, Asset: "ZAR"},
	"weird": {Code: 9999, Asset: "ZAR"},
}

// A committed transfer between mirrored accounts is queued for posting.
func TestCommittedTransferIsQueued(t *testing.T) {
	pool := setup(t)
	h := Handler(res)
	env := transfer(t, "tr-a", "pool", "cust", false, 10)
	apply(t, pool, func(ctx context.Context, tx pgx.Tx) error { return h(ctx, tx, env) })
	status, _, _, _ := store.Journal(context.Background(), pool, "tr-a")
	if status != store.StatusPending {
		t.Fatalf("status = %s, want PENDING", status)
	}
}

// A hold is not a posting, so it is not queued.
func TestPendingTransferIsNotQueued(t *testing.T) {
	pool := setup(t)
	env := transfer(t, "tr-hold", "pool", "cust", true, 11)
	apply(t, pool, func(ctx context.Context, tx pgx.Tx) error { return Handler(res)(ctx, tx, env) })
	if _, _, _, err := store.Journal(context.Background(), pool, "tr-hold"); err == nil {
		t.Fatal("a hold must not create a journal")
	}
}

// A code that core banking does not mirror is recorded as FAILED with the reason, for an operator. It is not dropped.
func TestUnmirroredCodeIsRecordedAsFailed(t *testing.T) {
	pool := setup(t)
	env := transfer(t, "tr-weird", "weird", "cust", false, 10)
	apply(t, pool, func(ctx context.Context, tx pgx.Tx) error { return Handler(res)(ctx, tx, env) })
	status, _, reason, _ := store.Journal(context.Background(), pool, "tr-weird")
	if status != store.StatusFailed || reason == "" {
		t.Fatalf("status=%s reason=%q", status, reason)
	}
}

// Redelivering the same transfer queues it once.
func TestRedeliveryQueuesOnce(t *testing.T) {
	pool := setup(t)
	env := transfer(t, "tr-b", "pool", "cust", false, 10)
	h := Handler(res)
	for i := 0; i < 2; i++ {
		apply(t, pool, func(ctx context.Context, tx pgx.Tx) error { return h(ctx, tx, env) })
	}
	var n int
	_ = pool.QueryRow(context.Background(), `select count(*) from cb_journal where transfer_id = 'tr-b'`).Scan(&n)
	if n != 1 {
		t.Fatalf("rows = %d, want 1", n)
	}
}

// An account the bridge does not know is an error, so the event is retried. The directory may simply be stale.
func TestUnknownAccountIsRetried(t *testing.T) {
	pool := setup(t)
	env := transfer(t, "tr-c", "nobody", "cust", false, 10)
	err := pgx.BeginFunc(context.Background(), pool, func(tx pgx.Tx) error { return Handler(res)(context.Background(), tx, env) })
	if err == nil {
		t.Fatal("an unknown account must be an error so the event is retried")
	}
}
