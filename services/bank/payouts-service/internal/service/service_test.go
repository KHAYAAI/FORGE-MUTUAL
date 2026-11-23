package service

import (
	"context"
	"fmt"
	"os"
	"testing"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	payoutsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/payouts/v1"
	"github.com/khayaai/forge-mutual/services/bank/payouts-service/internal/store"
)

// These tests need PostgreSQL. Set PAYOUTS_TEST_DSN to a throwaway database; the payout tables are truncated.

const (
	party    = "11111111-1111-4111-8111-111111111111"
	otherPty = "22222222-2222-4222-8222-222222222222"
	account  = "acc-zar-1"
)

type fakeOwners map[string]map[string]string

func (f fakeOwners) Accounts(_ context.Context, p string) (map[string]string, error) {
	return f[p], nil
}

// fakeRuns records every run started.
type fakeRuns struct{ started []string }

func (f *fakeRuns) StartRun(_ context.Context, batchID string, run uint32) error {
	f.started = append(f.started, fmt.Sprintf("%s:%d", batchID, run))
	return nil
}

func newService(t *testing.T) (*Service, *fakeRuns, *pgxpool.Pool) {
	t.Helper()
	dsn := os.Getenv("PAYOUTS_TEST_DSN")
	if dsn == "" {
		t.Skip("PAYOUTS_TEST_DSN not set")
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
	if _, err := pool.Exec(ctx, `truncate payout_item, payout_batch`); err != nil {
		t.Fatal(err)
	}
	runs := &fakeRuns{}
	return &Service{
		Pool:   pool,
		Owners: fakeOwners{party: {account: "ZAR"}},
		Runs:   runs,
		Limits: Limits{MaxItems: 5, MaxItemMinor: 1_000_000},
	}, runs, pool
}

func item(key, amount string) *payoutsv1.ItemInput {
	return &payoutsv1.ItemInput{
		ItemKey: key, AmountMinor: amount, BeneficiaryName: "Thandi Dube", AccountNumber: "1234567890",
		BankCode: "250655", Reference: "invoice 42",
	}
}

func create(key string, items ...*payoutsv1.ItemInput) *connect.Request[payoutsv1.CreateBatchRequest] {
	return connect.NewRequest(&payoutsv1.CreateBatchRequest{
		IdempotencyKey: key, PartyId: party, FromAccountId: account, Asset: "ZAR", Items: items, Actor: "maker",
	})
}

func mustCreate(t *testing.T, s *Service, items ...*payoutsv1.ItemInput) string {
	t.Helper()
	res, err := s.CreateBatch(context.Background(), create("batch-"+fmt.Sprint(len(items))+"-"+items[0].ItemKey, items...))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	return res.Msg.Batch.BatchId
}

func codeOf(err error) connect.Code { return connect.CodeOf(err) }

// The checker must not be the maker. A different person approves, and only then does the first run start.
func TestMakerCannotApprove(t *testing.T) {
	s, runs, _ := newService(t)
	ctx := context.Background()
	id := mustCreate(t, s, item("k1", "1000"))

	_, err := s.ApproveBatch(ctx, connect.NewRequest(&payoutsv1.ApproveBatchRequest{BatchId: id, Actor: "maker"}))
	if codeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("maker approving own batch: want PermissionDenied, got %v", err)
	}
	if len(runs.started) != 0 {
		t.Fatal("a refused approval must not start a run")
	}

	res, err := s.ApproveBatch(ctx, connect.NewRequest(&payoutsv1.ApproveBatchRequest{BatchId: id, Actor: "checker"}))
	if err != nil {
		t.Fatal(err)
	}
	if res.Msg.Batch.State != payoutsv1.BatchState_BATCH_STATE_RUNNING || res.Msg.Batch.ApprovedBy != "checker" || res.Msg.Batch.Run != 1 {
		t.Fatalf("approved batch = %+v", res.Msg.Batch)
	}
	if len(runs.started) != 1 || runs.started[0] != id+":1" {
		t.Fatalf("runs started = %v", runs.started)
	}
}

// The same key with the same request returns the batch already created. A different request under that key is refused.
func TestCreateIsIdempotent(t *testing.T) {
	s, _, _ := newService(t)
	ctx := context.Background()
	first, err := s.CreateBatch(ctx, create("key-1", item("k1", "1000")))
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.CreateBatch(ctx, create("key-1", item("k1", "1000")))
	if err != nil || again.Msg.Batch.BatchId != first.Msg.Batch.BatchId {
		t.Fatalf("replay: %v, %v", again, err)
	}
	_, err = s.CreateBatch(ctx, create("key-1", item("k1", "2000")))
	if codeOf(err) != connect.CodeAlreadyExists {
		t.Fatalf("same key, different request: want AlreadyExists, got %v", err)
	}
}

// A batch can only be paid from an account the party owns, and that holds the currency paid.
func TestUnownedAccountIsRefused(t *testing.T) {
	s, _, _ := newService(t)
	ctx := context.Background()
	req := connect.NewRequest(&payoutsv1.CreateBatchRequest{
		IdempotencyKey: "k", PartyId: otherPty, FromAccountId: account, Asset: "ZAR",
		Items: []*payoutsv1.ItemInput{item("k1", "1000")}, Actor: "maker",
	})
	if _, err := s.CreateBatch(ctx, req); codeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("account of another party: want PermissionDenied, got %v", err)
	}
}

func TestValidationRefusesBadBatches(t *testing.T) {
	s, _, _ := newService(t)
	ctx := context.Background()
	cases := map[string][]*payoutsv1.ItemInput{
		"duplicate key":   {item("k1", "1000"), item("k1", "2000")},
		"zero amount":     {item("k1", "0")},
		"leading zero":    {item("k1", "0100")},
		"above the limit": {item("k1", "1000001")},
		"too many items":  {item("a", "1"), item("b", "1"), item("c", "1"), item("d", "1"), item("e", "1"), item("f", "1")},
		"bad item key":    {item("has space", "1000")},
	}
	i := 0
	for name, items := range cases {
		i++
		_, err := s.CreateBatch(ctx, create(fmt.Sprintf("v-%d", i), items...))
		if codeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: want InvalidArgument, got %v", name, err)
		}
	}
	usd := connect.NewRequest(&payoutsv1.CreateBatchRequest{
		IdempotencyKey: "usd", PartyId: party, FromAccountId: account, Asset: "USD",
		Items: []*payoutsv1.ItemInput{item("k1", "1000")}, Actor: "maker",
	})
	if _, err := s.CreateBatch(ctx, usd); codeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("USD batch: want InvalidArgument, got %v", err)
	}
}

// A batch that ended with exceptions gets a new run when resumed. A batch still running has its current run started
// again. Resuming a batch that was never approved is refused, and a completed batch is not run again.
func TestResumeFollowsTheState(t *testing.T) {
	s, runs, pool := newService(t)
	ctx := context.Background()
	id := mustCreate(t, s, item("k1", "1000"))

	if _, err := s.ResumeBatch(ctx, connect.NewRequest(&payoutsv1.ResumeBatchRequest{BatchId: id, Actor: "ops"})); codeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("resume before approval: want FailedPrecondition, got %v", err)
	}
	if _, err := s.ApproveBatch(ctx, connect.NewRequest(&payoutsv1.ApproveBatchRequest{BatchId: id, Actor: "checker"})); err != nil {
		t.Fatal(err)
	}

	// The run ends with an item still open, so the batch ends with exceptions.
	if _, err := store.FinishRun(ctx, pool, id, 1); err != nil {
		t.Fatal(err)
	}
	res, err := s.ResumeBatch(ctx, connect.NewRequest(&payoutsv1.ResumeBatchRequest{BatchId: id, Actor: "ops"}))
	if err != nil {
		t.Fatal(err)
	}
	if res.Msg.Batch.Run != 2 || res.Msg.Batch.State != payoutsv1.BatchState_BATCH_STATE_RUNNING {
		t.Fatalf("resumed batch = %+v", res.Msg.Batch)
	}
	if runs.started[len(runs.started)-1] != id+":2" {
		t.Fatalf("runs started = %v; resume must start run 2", runs.started)
	}

	// Every item settles, so the batch completes and a further resume does nothing.
	if err := store.SetItem(ctx, pool, id, "k1", store.ItemSettled, "p1", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := store.FinishRun(ctx, pool, id, 2); err != nil {
		t.Fatal(err)
	}
	before := len(runs.started)
	done, err := s.ResumeBatch(ctx, connect.NewRequest(&payoutsv1.ResumeBatchRequest{BatchId: id, Actor: "ops"}))
	if err != nil || done.Msg.Batch.State != payoutsv1.BatchState_BATCH_STATE_COMPLETED {
		t.Fatalf("resume of a completed batch: %+v, %v", done, err)
	}
	if len(runs.started) != before {
		t.Fatal("a completed batch must not start another run")
	}
}

// Read back, an item shows its state, and the batch total is the sum of its items.
func TestGetReturnsItemsAndTotal(t *testing.T) {
	s, _, _ := newService(t)
	ctx := context.Background()
	id := mustCreate(t, s, item("k1", "1000"), item("k2", "250"))
	res, err := s.GetBatch(ctx, connect.NewRequest(&payoutsv1.GetBatchRequest{BatchId: id}))
	if err != nil {
		t.Fatal(err)
	}
	b := res.Msg.Batch
	if b.TotalMinor != "1250" || len(b.Items) != 2 || b.Items[0].State != payoutsv1.ItemState_ITEM_STATE_PENDING {
		t.Fatalf("batch = %+v", b)
	}
	if _, err := s.GetBatch(ctx, connect.NewRequest(&payoutsv1.GetBatchRequest{BatchId: "11111111-1111-4111-8111-999999999999"})); codeOf(err) != connect.CodeNotFound {
		t.Fatalf("missing batch: want NotFound, got %v", err)
	}
}
