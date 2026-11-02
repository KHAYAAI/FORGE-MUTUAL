package onboarding

import (
	"context"
	"testing"
	"time"

	"go.temporal.io/sdk/testsuite"
)

// OpenKYCReview is the activity a provider's "could not decide" verdict triggers. The fake records each call.
func (f *fakeActivities) OpenKYCReview(_ context.Context, _ Input, _ string) error {
	f.note("OpenKYCReview")
	return nil
}

// runWithReviews executes the workflow. Verdicts arrive at one minute; the reviews arrive one per minute from two.
func runWithReviews(t *testing.T, fake *fakeActivities, verdicts map[string]string, reviews []KYCReviewSignal, timeoutSeconds int64) Status {
	t.Helper()
	var ts testsuite.WorkflowTestSuite
	env := ts.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(OnboardBusiness)
	env.RegisterActivity(fake)
	for subject, verdict := range verdicts {
		subject, verdict := subject, verdict
		env.RegisterDelayedCallback(func() {
			env.SignalWorkflow(SignalKYCResult, KYCSignal{SubjectPartyID: subject, Verdict: verdict})
		}, time.Minute)
	}
	for i, rv := range reviews {
		rv := rv
		env.RegisterDelayedCallback(func() {
			env.SignalWorkflow(SignalKYCReview, rv)
		}, time.Duration(2+i)*time.Minute)
	}
	in := input()
	in.KYCTimeoutSeconds = timeoutSeconds
	env.ExecuteWorkflow(OnboardBusiness, in)
	if !env.IsWorkflowCompleted() {
		t.Fatal("the workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	var st Status
	if err := env.GetWorkflowResult(&st); err != nil {
		t.Fatal(err)
	}
	return st
}

// A provider that cannot decide queues a review. A person's approval resumes the application, which then completes.
func TestPersonApprovalResumesAReviewedApplication(t *testing.T) {
	fake := &fakeActivities{sectorAllowed: true, calls: map[string]int{}}
	st := runWithReviews(t, fake,
		map[string]string{"company-1": "APPROVED", "person-1": "REVIEW", "person-2": "APPROVED"},
		[]KYCReviewSignal{{SubjectPartyID: "person-1", Decision: "APPROVED", Reviewer: "naledi", Note: "checked against the register"}},
		0)
	if st.DeclineReason != "" || fake.calls["OpenKYCReview"] != 1 || fake.calls["CreateAccounts"] != 1 {
		t.Fatalf("status = %+v, calls = %v", st, fake.calls)
	}
}

// A person's decline ends the application, and creates no account.
func TestPersonDeclineEndsTheApplication(t *testing.T) {
	fake := &fakeActivities{sectorAllowed: true, calls: map[string]int{}}
	st := runWithReviews(t, fake,
		map[string]string{"company-1": "APPROVED", "person-1": "REVIEW", "person-2": "APPROVED"},
		[]KYCReviewSignal{{SubjectPartyID: "person-1", Decision: "DECLINED", Reviewer: "naledi", Note: "the ID does not match"}},
		0)
	if st.State != StateDeclined || st.DeclineReason != "kyc_review_declined:person-1" {
		t.Fatalf("status = %+v", st)
	}
	if fake.calls["CreateAccounts"] != 0 {
		t.Fatal("a declined review must not create a ledger account")
	}
}

// A review nobody decides does not wait forever: the same timeout as any missing verdict declines it.
func TestUndecidedReviewTimesOut(t *testing.T) {
	fake := &fakeActivities{sectorAllowed: true, calls: map[string]int{}}
	st := runWithReviews(t, fake,
		map[string]string{"company-1": "APPROVED", "person-1": "REVIEW", "person-2": "APPROVED"},
		nil, 300)
	if st.State != StateDeclined || st.DeclineReason != "kyc_timeout" {
		t.Fatalf("status = %+v", st)
	}
}

// A decision for a subject nobody is reviewing is ignored, so a stray or replayed decision cannot pass or fail a check.
// Two reviews are then decided in turn, and the application completes.
func TestDecisionsForUnreviewedSubjectsAreIgnored(t *testing.T) {
	fake := &fakeActivities{sectorAllowed: true, calls: map[string]int{}}
	st := runWithReviews(t, fake,
		map[string]string{"company-1": "REVIEW", "person-1": "REVIEW", "person-2": "REVIEW"},
		[]KYCReviewSignal{
			{SubjectPartyID: "person-9", Decision: "DECLINED", Reviewer: "naledi", Note: "not a subject of this application"},
			{SubjectPartyID: "company-1", Decision: "APPROVED", Reviewer: "naledi", Note: "checked against the register"},
			{SubjectPartyID: "person-1", Decision: "APPROVED", Reviewer: "sipho", Note: "checked against the ID document"},
			{SubjectPartyID: "person-2", Decision: "APPROVED", Reviewer: "sipho", Note: "checked against the ID document"},
		},
		0)
	if st.DeclineReason != "" || fake.calls["OpenKYCReview"] != 3 {
		t.Fatalf("status = %+v, calls = %v", st, fake.calls)
	}
}
