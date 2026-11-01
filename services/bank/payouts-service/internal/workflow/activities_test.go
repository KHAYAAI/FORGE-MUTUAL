package workflow

import (
	"context"
	"errors"
	"os"
	"testing"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	paymentsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/payments/v1"
	"github.com/khayaai/forge-mutual/pkg/uuid"
	"github.com/khayaai/forge-mutual/services/bank/payouts-service/internal/store"
)

// These tests need PostgreSQL. Set PAYOUTS_TEST_DSN to a throwaway database; the payout tables are truncated.

// fakePayments records every request and answers from a script.
type fakePayments struct {
	creates   []*paymentsv1.CreatePaymentRequest
	createErr []error // consumed in order; a nil entry succeeds
	nextID    string
	states    map[string]paymentsv1.PaymentState
}

func (f *fakePayments) CreatePayment(_ context.Context, req *paymentsv1.CreatePaymentRequest) (string, error) {
	f.creates = append(f.creates, req)
	if len(f.createErr) > 0 {
		err := f.createErr[0]
		f.createErr = f.createErr[1:]
		if err != nil {
			return "", err
		}
	}
	return f.nextID, nil
}

func (f *fakePayments) GetPayment(_ context.Context, id string) (paymentsv1.PaymentState, error) {
	return f.states[id], nil
}

const testParty = "11111111-1111-4111-8111-111111111111"

func openDB(t *testing.T) *pgxpool.Pool {
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
	return pool
}

// seedBatch stores a batch with one item per key, in the DRAFT state.
func seedBatch(t *testing.T, pool *pgxpool.Pool, keys ...string) string {
	t.Helper()
	id, err := uuid.NewV4()
	if err != nil {
		t.Fatal(err)
	}
	items := make([]store.Item, 0, len(keys))
	for i, k := range keys {
		items = append(items, store.Item{
			Key: k, Seq: i + 1, AmountMinor: 1000, BeneficiaryName: "Thandi Dube",
			AccountNumber: "1234567890", BankCode: "250655", Reference: "invoice 42",
		})
	}
	b, _, err := store.Create(context.Background(), pool, store.Batch{
		ID: id, IdempotencyKey: "seed-" + id, RequestHash: []byte("h"), PartyID: testParty,
		FromAccountID: "acc-1", Asset: "ZAR", TotalMinor: int64(1000 * len(keys)), CreatedBy: "maker", Items: items,
	})
	if err != nil {
		t.Fatal(err)
	}
	return b.ID
}

func itemState(t *testing.T, pool *pgxpool.Pool, batchID, key string) store.Item {
	t.Helper()
	it, err := store.GetItem(context.Background(), pool, batchID, key)
	if err != nil {
		t.Fatal(err)
	}
	return it
}

// A final refusal marks the item failed. It is never sent again.
func TestRefusalIsFinal(t *testing.T) {
	pool := openDB(t)
	ctx := context.Background()
	batch := seedBatch(t, pool, "k1")
	fake := &fakePayments{createErr: []error{connect.NewError(connect.CodeInvalidArgument, errors.New("bad account"))}}
	acts := &Activities{Pool: pool, Payments: fake}

	first, err := acts.SendItem(ctx, SendInput{BatchID: batch, ItemKey: "k1"})
	if err != nil || first.Outcome != "refused" {
		t.Fatalf("first send: %+v, %v", first, err)
	}
	second, err := acts.SendItem(ctx, SendInput{BatchID: batch, ItemKey: "k1"})
	if err != nil || second.Outcome != "failed" {
		t.Fatalf("second send: %+v, %v", second, err)
	}
	if len(fake.creates) != 1 {
		t.Fatalf("payments was called %d times; a refused item must not be sent again", len(fake.creates))
	}
	if got := itemState(t, pool, batch, "k1"); got.State != store.ItemFailed {
		t.Fatalf("item state = %s, want FAILED", got.State)
	}
}

// When payments cannot be reached, the send errors so Temporal retries it. The retry carries the same key, so payments
// answers it as a replay if the first attempt had taken the payment.
func TestUnreachableSendRetriesUnderTheSameKey(t *testing.T) {
	pool := openDB(t)
	ctx := context.Background()
	batch := seedBatch(t, pool, "k1")
	fake := &fakePayments{
		createErr: []error{connect.NewError(connect.CodeUnavailable, errors.New("down")), nil},
		nextID:    "11111111-2222-4333-8444-555555555555",
	}
	acts := &Activities{Pool: pool, Payments: fake}

	if _, err := acts.SendItem(ctx, SendInput{BatchID: batch, ItemKey: "k1"}); err == nil {
		t.Fatal("an unreachable send must return an error so that it is retried")
	}
	res, err := acts.SendItem(ctx, SendInput{BatchID: batch, ItemKey: "k1"})
	if err != nil || res.Outcome != "sent" || res.PaymentID != fake.nextID {
		t.Fatalf("retry: %+v, %v", res, err)
	}
	if len(fake.creates) != 2 || fake.creates[0].IdempotencyKey != fake.creates[1].IdempotencyKey {
		t.Fatal("the retry must use the same idempotency key as the first attempt")
	}
	if fake.creates[0].IdempotencyKey != PaymentKey(batch, "k1") {
		t.Fatalf("key = %s, want the batch-and-item key", fake.creates[0].IdempotencyKey)
	}
	if got := itemState(t, pool, batch, "k1"); got.State != store.ItemSent || got.PaymentID != fake.nextID {
		t.Fatalf("item = %+v, want SENT with the payment id", got)
	}
}

// An item already sent is not sent again: its existing payment is returned.
func TestSentItemIsNotSentAgain(t *testing.T) {
	pool := openDB(t)
	ctx := context.Background()
	batch := seedBatch(t, pool, "k1")
	fake := &fakePayments{nextID: "11111111-2222-4333-8444-555555555555"}
	acts := &Activities{Pool: pool, Payments: fake}

	if _, err := acts.SendItem(ctx, SendInput{BatchID: batch, ItemKey: "k1"}); err != nil {
		t.Fatal(err)
	}
	again, err := acts.SendItem(ctx, SendInput{BatchID: batch, ItemKey: "k1"})
	if err != nil || again.Outcome != "sent" || again.PaymentID != fake.nextID {
		t.Fatalf("second send: %+v, %v", again, err)
	}
	if len(fake.creates) != 1 {
		t.Fatalf("payments was called %d times for one item", len(fake.creates))
	}
}

// The payment's state decides the item. Only a completed payment settles it; a declined, failed or cancelled one fails
// it; anything still in progress leaves it sent.
func TestPollMapsPaymentStates(t *testing.T) {
	pool := openDB(t)
	ctx := context.Background()
	batch := seedBatch(t, pool, "a", "b", "c")
	fake := &fakePayments{states: map[string]paymentsv1.PaymentState{
		"pa": paymentsv1.PaymentState_PAYMENT_STATE_COMPLETED,
		"pb": paymentsv1.PaymentState_PAYMENT_STATE_DECLINED,
		"pc": paymentsv1.PaymentState_PAYMENT_STATE_REPAIR_REQUIRED,
	}}
	acts := &Activities{Pool: pool, Payments: fake}
	for key, id := range map[string]string{"a": "pa", "b": "pb", "c": "pc"} {
		if err := store.SetItem(ctx, pool, batch, key, store.ItemSent, id, ""); err != nil {
			t.Fatal(err)
		}
	}

	want := map[string]string{"a": "settled", "b": "failed", "c": "pending"}
	for key, id := range map[string]string{"a": "pa", "b": "pb", "c": "pc"} {
		res, err := acts.PollItem(ctx, PollInput{BatchID: batch, ItemKey: key, PaymentID: id})
		if err != nil || res.Outcome != want[key] {
			t.Fatalf("item %s: %+v, %v; want %s", key, res, err, want[key])
		}
	}
	if got := itemState(t, pool, batch, "a"); got.State != store.ItemSettled {
		t.Fatalf("a = %s, want SETTLED", got.State)
	}
	if got := itemState(t, pool, batch, "c"); got.State != store.ItemSent {
		t.Fatalf("c = %s, want SENT while in progress", got.State)
	}
}

// A settled item is final: a late write that says it was sent again does not move it back.
func TestFinalItemDoesNotMoveBack(t *testing.T) {
	pool := openDB(t)
	ctx := context.Background()
	batch := seedBatch(t, pool, "k1")
	if err := store.SetItem(ctx, pool, batch, "k1", store.ItemSettled, "p1", ""); err != nil {
		t.Fatal(err)
	}
	if err := store.SetItem(ctx, pool, batch, "k1", store.ItemSent, "p2", ""); err != nil {
		t.Fatal(err)
	}
	if got := itemState(t, pool, batch, "k1"); got.State != store.ItemSettled || got.PaymentID != "p1" {
		t.Fatalf("item = %+v; a settled item must keep its state and payment", got)
	}
}
