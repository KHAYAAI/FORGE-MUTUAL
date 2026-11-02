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
