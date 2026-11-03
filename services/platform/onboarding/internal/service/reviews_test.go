package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	onboardingv1 "github.com/khayaai/forge-mutual/pkg/api/forge/onboarding/v1"
	"github.com/khayaai/forge-mutual/services/platform/onboarding/internal/onboarding"
	"github.com/khayaai/forge-mutual/services/platform/onboarding/internal/store"
	"github.com/khayaai/forge-mutual/services/platform/onboarding/internal/testdb"
)

// These tests need PostgreSQL. Set ONBOARDING_TEST_PG to a base DSN; each test gets its own fresh database.

const note = "Checked the register extract and the ID document against the applicant."

func reviewRig(t *testing.T) (*Service, *fakeTemporal, *store.Store, string) {
	t.Helper()
	dsn := testdb.Scratch(t, "ONBOARDING_TEST_PG")
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

// The decision is recorded before the workflow is told. If Temporal is down, the decision stays on the record,
// undelivered, and the reviewer is told it will be retried. Nothing is lost and nothing is decided twice.
func TestFailedSignalKeepsTheRecordedDecision(t *testing.T) {
	s, tc, st, id := reviewRig(t)
	ctx := context.Background()
	tc.signalErr = connect.NewError(connect.CodeUnavailable, context.DeadlineExceeded)
	if _, err := s.DecideKYCReview(ctx, decide(id, "APPROVED", "naledi", note)); connect.CodeOf(err) != connect.CodeUnavailable {
		t.Fatalf("want Unavailable, got %v", err)
	}
	r, err := st.Get(ctx, id)
	if err != nil || r.Status != "DECIDED" || r.Decision != "APPROVED" || r.Reviewer != "naledi" {
		t.Fatalf("review = %+v, %v; the decision must be on the record", r, err)
	}
	if r.SignalledAt != nil {
		t.Fatal("the workflow was not told, so the decision must not be marked delivered")
	}
	// It is not in the open queue, and it is waiting for delivery.
	open, _ := st.List(ctx, true)
	waiting, _ := st.Undelivered(ctx, 10)
	if len(open) != 0 || len(waiting) != 1 {
		t.Fatalf("open = %d, undelivered = %d", len(open), len(waiting))
	}
}

// The same decision again finishes the delivery once Temporal is back, and then repeats without telling the workflow
// again: a retry whose response was lost is safe.
func TestRetryingTheSameDecisionFinishesDelivery(t *testing.T) {
	s, tc, st, id := reviewRig(t)
	ctx := context.Background()
	tc.signalErr = connect.NewError(connect.CodeUnavailable, context.DeadlineExceeded)
	if _, err := s.DecideKYCReview(ctx, decide(id, "APPROVED", "naledi", note)); err == nil {
		t.Fatal("the first attempt should report that delivery failed")
	}
	tc.signalErr = nil
	res, err := s.DecideKYCReview(ctx, decide(id, "APPROVED", "naledi", note))
	if err != nil || res.Msg.Review.Status != "DECIDED" {
		t.Fatalf("retry = %+v, %v", res, err)
	}
	if len(tc.reviewSignals) != 1 {
		t.Fatalf("the workflow was told %d times, want once", len(tc.reviewSignals))
	}
	if r, _ := st.Get(ctx, id); r.SignalledAt == nil {
		t.Fatal("a delivered decision must be marked")
	}
	// Once more: nothing changes, nothing is sent.
	if _, err := s.DecideKYCReview(ctx, decide(id, "APPROVED", "naledi", note)); err != nil {
		t.Fatalf("repeat after delivery: %v", err)
	}
	if len(tc.reviewSignals) != 1 {
		t.Fatalf("the workflow was told %d times after a repeat, want once", len(tc.reviewSignals))
	}
}

// A different decision on a recorded review is refused, even while delivery of the first is still pending.
func TestADifferentDecisionOnARecordedReviewIsRefused(t *testing.T) {
	s, tc, st, id := reviewRig(t)
	ctx := context.Background()
	tc.signalErr = connect.NewError(connect.CodeUnavailable, context.DeadlineExceeded)
	_, _ = s.DecideKYCReview(ctx, decide(id, "APPROVED", "naledi", note))
	tc.signalErr = nil
	for name, req := range map[string]*connect.Request[onboardingv1.DecideKYCReviewRequest]{
		"other decision": decide(id, "DECLINED", "naledi", note),
		"other reviewer": decide(id, "APPROVED", "sipho", note),
		"other note":     decide(id, "APPROVED", "naledi", "A different account of what was checked, in full."),
	} {
		if _, err := s.DecideKYCReview(ctx, req); connect.CodeOf(err) != connect.CodeFailedPrecondition {
			t.Errorf("%s: want FailedPrecondition, got %v", name, err)
		}
	}
	if len(tc.reviewSignals) != 0 {
		t.Fatal("a refused decision must not reach the workflow")
	}
	if r, _ := st.Get(ctx, id); r.Decision != "APPROVED" || r.Reviewer != "naledi" {
		t.Fatalf("the recorded decision changed: %+v", r)
	}
}

// If the decision cannot be recorded, the workflow is not told: it never hears of a decision that is not on the record.
type failingDecide struct{ ReviewQueue }

func (failingDecide) Decide(context.Context, string, string, string, string, time.Time) error {
	return errors.New("database down")
}

func TestAnUnrecordedDecisionNeverReachesTheWorkflow(t *testing.T) {
	s, tc, st, id := reviewRig(t)
	s.WithReviews(failingDecide{st})
	if _, err := s.DecideKYCReview(context.Background(), decide(id, "APPROVED", "naledi", note)); connect.CodeOf(err) != connect.CodeInternal {
		t.Fatalf("want Internal, got %v", err)
	}
	if len(tc.reviewSignals) != 0 {
		t.Fatal("the workflow was told of a decision that was not recorded")
	}
	if r, _ := st.Get(context.Background(), id); r.Status != "OPEN" {
		t.Fatalf("the review must stay open, got %s", r.Status)
	}
}

// When the onboarding has already ended there is nothing to resume. The decision stays on the record, the reviewer is
// told so, and the decision is not retried for ever.
func TestADecisionForAnEndedOnboardingIsRecordedAndSettled(t *testing.T) {
	s, tc, st, id := reviewRig(t)
	ctx := context.Background()
	delete(tc.started, "onb-1")
	if _, err := s.DecideKYCReview(ctx, decide(id, "APPROVED", "naledi", note)); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("want FailedPrecondition, got %v", err)
	}
	r, _ := st.Get(ctx, id)
	if r.Status != "DECIDED" || r.SignalledAt == nil {
		t.Fatalf("review = %+v; the decision must be recorded and settled", r)
	}
	if n, err := s.RedeliverDecisions(ctx); n != 0 || err != nil {
		t.Fatalf("redelivery = %d, %v; a settled decision must not be retried", n, err)
	}
}

// The redelivery loop tells the workflow of decisions it missed, once, and then has nothing left to do.
func TestRedeliveryTellsTheWorkflowOfMissedDecisions(t *testing.T) {
	s, tc, st, id := reviewRig(t)
	ctx := context.Background()
	tc.signalErr = connect.NewError(connect.CodeUnavailable, context.DeadlineExceeded)
	_, _ = s.DecideKYCReview(ctx, decide(id, "DECLINED", "naledi", note))

	// While Temporal is still down, redelivery reports it and leaves the decision waiting.
	if n, err := s.RedeliverDecisions(ctx); n != 0 || connect.CodeOf(err) != connect.CodeUnavailable {
		t.Fatalf("during the outage: told %d, err %v", n, err)
	}
	tc.signalErr = nil
	n, err := s.RedeliverDecisions(ctx)
	if n != 1 || err != nil {
		t.Fatalf("after the outage: told %d, err %v", n, err)
	}
	if len(tc.reviewSignals) != 1 || tc.reviewSignals[0].Decision != "DECLINED" || tc.reviewSignals[0].Reviewer != "naledi" {
		t.Fatalf("signals = %+v", tc.reviewSignals)
	}
	if n, err := s.RedeliverDecisions(ctx); n != 0 || err != nil {
		t.Fatalf("second pass: told %d, err %v", n, err)
	}
	if r, _ := st.Get(ctx, id); r.SignalledAt == nil {
		t.Fatal("the delivered decision must be marked")
	}
}

// Two reviewers deciding at the same moment: one decision is recorded, the other is refused, and the workflow is told
// exactly once, with the recorded decision.
func TestTwoReviewersAtOnceTellTheWorkflowOnce(t *testing.T) {
	s, tc, st, id := reviewRig(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, who := range []string{"naledi", "sipho"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = s.DecideKYCReview(ctx, decide(id, []string{"APPROVED", "DECLINED"}[i], who, note))
		}()
	}
	wg.Wait()
	refused := 0
	for _, err := range errs {
		if connect.CodeOf(err) == connect.CodeFailedPrecondition {
			refused++
		}
	}
	if refused != 1 {
		t.Fatalf("errors = %v; exactly one reviewer should be refused", errs)
	}
	r, _ := st.Get(ctx, id)
	if len(tc.reviewSignals) != 1 || tc.reviewSignals[0].Decision != r.Decision || tc.reviewSignals[0].Reviewer != r.Reviewer {
		t.Fatalf("signals = %+v, recorded = %s by %s", tc.reviewSignals, r.Decision, r.Reviewer)
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
