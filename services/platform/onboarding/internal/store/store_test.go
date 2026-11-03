package store

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/khayaai/forge-mutual/services/platform/onboarding/internal/testdb"
)

// These tests need PostgreSQL. Set ONBOARDING_TEST_PG to a base DSN; each test gets its own fresh database.

var now = time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)

func openTest(t *testing.T) *Store {
	t.Helper()
	dsn := testdb.Scratch(t, "ONBOARDING_TEST_PG")
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	st, err := Open(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

// A subject has one open review: a repeat verdict returns the same review and queues no second one.
func TestOneOpenReviewPerSubject(t *testing.T) {
	ctx := context.Background()
	st := openTest(t)
	a, err := st.OpenReview(ctx, "onb-1", "person-1", "provider requested review", now)
	if err != nil {
		t.Fatal(err)
	}
	b, err := st.OpenReview(ctx, "onb-1", "person-1", "provider requested review", now.Add(time.Minute))
	if err != nil || a != b {
		t.Fatalf("repeat: %s vs %s, %v", a, b, err)
	}
	all, err := st.List(ctx, false)
	if err != nil || len(all) != 1 {
		t.Fatalf("reviews = %d, %v", len(all), err)
	}
}

// A decision is recorded once. A second decision on the same review is refused, and the first stands.
func TestDecisionIsRecordedOnce(t *testing.T) {
	ctx := context.Background()
	st := openTest(t)
	id, err := st.OpenReview(ctx, "onb-2", "person-2", "provider requested review", now)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Decide(ctx, id, "APPROVED", "naledi", "Documents match the register; checked against the ID.", now); err != nil {
		t.Fatal(err)
	}
	if err := st.Decide(ctx, id, "DECLINED", "sipho", "A second opinion that should not be recorded.", now); err != ErrNotOpen {
		t.Fatalf("second decision: %v, want ErrNotOpen", err)
	}
	r, err := st.Get(ctx, id)
	if err != nil || r.Status != "DECIDED" || r.Decision != "APPROVED" || r.Reviewer != "naledi" {
		t.Fatalf("review = %+v, %v", r, err)
	}
	open, err := st.List(ctx, true)
	if err != nil || len(open) != 0 {
		t.Fatalf("open reviews after the decision = %d, %v", len(open), err)
	}
}

func TestMissingReviewIsNotFound(t *testing.T) {
	st := openTest(t)
	if _, err := st.Get(context.Background(), "11111111-1111-4111-8111-999999999999"); err != ErrNotFound {
		t.Fatalf("missing review: %v", err)
	}
}

// A decision waits for delivery until the workflow is marked as told. Marking is safe to repeat, and an open review is
// never waiting for delivery.
func TestADecidedReviewWaitsForDeliveryUntilMarked(t *testing.T) {
	ctx := context.Background()
	st := openTest(t)
	id, err := st.OpenReview(ctx, "onb-1", "person-1", "provider requested review", now)
	if err != nil {
		t.Fatal(err)
	}
	if un, err := st.Undelivered(ctx, 10); err != nil || len(un) != 0 {
		t.Fatalf("an open review is not waiting for delivery: %v, %v", un, err)
	}
	if err := st.MarkSignalled(ctx, id, now); err != nil {
		t.Fatal(err)
	}
	if r, _ := st.Get(ctx, id); r.SignalledAt != nil {
		t.Fatal("an open review cannot be marked delivered")
	}

	if err := st.Decide(ctx, id, "APPROVED", "naledi", "Checked the register extract and the ID.", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	un, err := st.Undelivered(ctx, 10)
	if err != nil || len(un) != 1 || un[0].ReviewID != id || un[0].SignalledAt != nil {
		t.Fatalf("a decided review waits for delivery: %+v, %v", un, err)
	}
	if err := st.MarkSignalled(ctx, id, now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	first, _ := st.Get(ctx, id)
	if first.SignalledAt == nil {
		t.Fatal("marking must record when the workflow was told")
	}
	if err := st.MarkSignalled(ctx, id, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if again, _ := st.Get(ctx, id); !again.SignalledAt.Equal(*first.SignalledAt) {
		t.Fatal("marking twice must keep the first time")
	}
	if un, _ := st.Undelivered(ctx, 10); len(un) != 0 {
		t.Fatalf("a delivered decision is no longer waiting: %v", un)
	}
}

// A table made before delivery was tracked is upgraded in place by Open, and Open can be run again.
func TestOpenUpgradesATableMadeBeforeDeliveryWasTracked(t *testing.T) {
	dsn := testdb.Scratch(t, "ONBOARDING_TEST_PG")
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `
		create table kyc_review (
		  review_id text primary key, onboarding_id text not null, subject_party_id text not null, reason text not null,
		  status text not null check (status in ('OPEN', 'DECIDED')), opened_at timestamptz not null,
		  decision text not null default '' check (decision in ('', 'APPROVED', 'DECLINED')),
		  reviewer text not null default '', note text not null default '', decided_at timestamptz);
		insert into kyc_review values ('old-1', 'onb-0', 'person-0', 'provider requested review', 'DECIDED', now(), 'APPROVED', 'naledi', 'Checked it.', now())`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		st, err := Open(ctx, pool)
		if err != nil {
			t.Fatalf("open %d: %v", i+1, err)
		}
		// The old decided row has no delivery time, so it is told to the workflow once more. That is harmless.
		un, err := st.Undelivered(ctx, 10)
		if err != nil || len(un) != 1 || un[0].ReviewID != "old-1" {
			t.Fatalf("open %d: undelivered = %+v, %v", i+1, un, err)
		}
	}
}
