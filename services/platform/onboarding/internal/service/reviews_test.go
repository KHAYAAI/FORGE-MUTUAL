package service

import (
	"context"
	"os"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	onboardingv1 "github.com/khayaai/forge-mutual/pkg/api/forge/onboarding/v1"
	"github.com/khayaai/forge-mutual/services/platform/onboarding/internal/onboarding"
	"github.com/khayaai/forge-mutual/services/platform/onboarding/internal/store"
)

// These tests need PostgreSQL. Set ONBOARDING_TEST_PG to a throwaway database; kyc_review is truncated.

const note = "Checked the register extract and the ID document against the applicant."

func reviewRig(t *testing.T) (*Service, *fakeTemporal, *store.Store, string) {
	t.Helper()
	dsn := os.Getenv("ONBOARDING_TEST_PG")
	if dsn == "" {
		t.Skip("ONBOARDING_TEST_PG not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	st, err := store.Open(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `truncate kyc_review`); err != nil {
		t.Fatal(err)
	}
	tc := newFakeTemporal()
	tc.started["onb-1"] = onboarding.Input{OnboardingID: "onb-1"}
	id, err := st.OpenReview(ctx, "onb-1", "person-1", "provider could not decide", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return New(tc, mustFake(t), "onboarding").WithReviews(st), tc, st, id
}

func decide(id, decision, reviewer, note string) *connect.Request[onboardingv1.DecideKYCReviewRequest] {
	return connect.NewRequest(&onboardingv1.DecideKYCReviewRequest{ReviewId: id, Decision: decision, Reviewer: reviewer, Note: note})
}

// A decision signals the waiting onboarding with the subject and the reviewer's note, and then records the decision.
func TestDecisionSignalsTheOnboardingAndIsRecorded(t *testing.T) {
	s, tc, st, id := reviewRig(t)
	res, err := s.DecideKYCReview(context.Background(), decide(id, "APPROVED", "naledi", note))
	if err != nil {
		t.Fatal(err)
	}
	if len(tc.reviewSignals) != 1 || tc.reviewSignals[0].SubjectPartyID != "person-1" || tc.reviewSignals[0].Decision != "APPROVED" {
		t.Fatalf("signals = %+v", tc.reviewSignals)
	}
	if res.Msg.Review.Status != "DECIDED" || res.Msg.Review.Reviewer != "naledi" || res.Msg.Review.DecidedAt == "" {
		t.Fatalf("review = %+v", res.Msg.Review)
	}
	r, err := st.Get(context.Background(), id)
	if err != nil || r.Decision != "APPROVED" {
		t.Fatalf("stored = %+v, %v", r, err)
	}
}

// A decision needs a real note, a known decision, and a reviewer. Anything else is refused before the workflow is told.
func TestDecisionNeedsANoteAndAKnownDecision(t *testing.T) {
	s, tc, _, id := reviewRig(t)
	ctx := context.Background()
	cases := map[string]*connect.Request[onboardingv1.DecideKYCReviewRequest]{
		"short note":    decide(id, "APPROVED", "naledi", "ok"),
		"unknown":       decide(id, "MAYBE", "naledi", note),
		"no reviewer":   decide(id, "APPROVED", "", note),
		"bad review id": decide("not-a-uuid", "APPROVED", "naledi", note),
	}
	for name, req := range cases {
		if _, err := s.DecideKYCReview(ctx, req); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: want InvalidArgument, got %v", name, err)
		}
	}
	if len(tc.reviewSignals) != 0 {
		t.Fatal("a refused decision must not reach the workflow")
	}
}

// If the workflow cannot be told, the review stays open, so the decision can be made again.
func TestFailedSignalLeavesTheReviewOpen(t *testing.T) {
	s, tc, st, id := reviewRig(t)
	tc.signalErr = connect.NewError(connect.CodeUnavailable, context.DeadlineExceeded)
	if _, err := s.DecideKYCReview(context.Background(), decide(id, "APPROVED", "naledi", note)); connect.CodeOf(err) != connect.CodeUnavailable {
		t.Fatalf("want Unavailable, got %v", err)
	}
	r, err := st.Get(context.Background(), id)
	if err != nil || r.Status != "OPEN" {
		t.Fatalf("review = %+v, %v; it must stay open", r, err)
	}
}

// A decided review cannot be decided again, and the second attempt does not reach the workflow.
func TestDecisionIsMadeOnce(t *testing.T) {
	s, tc, _, id := reviewRig(t)
	ctx := context.Background()
	if _, err := s.DecideKYCReview(ctx, decide(id, "APPROVED", "naledi", note)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DecideKYCReview(ctx, decide(id, "DECLINED", "sipho", note)); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("second decision: want FailedPrecondition, got %v", err)
	}
	if len(tc.reviewSignals) != 1 {
		t.Fatalf("the workflow was told %d times; a decided review must not signal again", len(tc.reviewSignals))
	}
}

func TestListReturnsOpenReviewsOldestFirst(t *testing.T) {
	s, _, _, id := reviewRig(t)
	ctx := context.Background()
	res, err := s.ListKYCReviews(ctx, connect.NewRequest(&onboardingv1.ListKYCReviewsRequest{OpenOnly: true}))
	if err != nil || len(res.Msg.Reviews) != 1 || res.Msg.Reviews[0].ReviewId != id {
		t.Fatalf("open reviews = %+v, %v", res, err)
	}
	if _, err := s.DecideKYCReview(ctx, decide(id, "DECLINED", "naledi", note)); err != nil {
		t.Fatal(err)
	}
	res, err = s.ListKYCReviews(ctx, connect.NewRequest(&onboardingv1.ListKYCReviewsRequest{OpenOnly: true}))
	if err != nil || len(res.Msg.Reviews) != 0 {
		t.Fatalf("open reviews after the decision = %+v, %v", res, err)
	}
}
