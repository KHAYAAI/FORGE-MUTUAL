package onboarding

import (
	"context"
	"testing"
	"time"

	"go.temporal.io/sdk/testsuite"
)

func (f *fakeActivities) CreatePerson(_ context.Context, _ PersonInput) (string, error) {
	f.note("CreatePerson")
	return "person-9", nil
}

func (f *fakeActivities) RecordPersonConsents(_ context.Context, _ PersonInput, _ string) error {
	f.note("RecordPersonConsents")
	return nil
}

func (f *fakeActivities) CreatePersonAccount(_ context.Context, _ PersonInput, _ string) ([]string, error) {
	f.note("CreatePersonAccount")
	if f.accountErr != nil {
		return nil, f.accountErr
	}
	return []string{"account-9"}, nil
}

func runPerson(t *testing.T, fake *fakeActivities, signals func(env *testsuite.TestWorkflowEnvironment)) (Status, error) {
	t.Helper()
	var ts testsuite.WorkflowTestSuite
	env := ts.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(OnboardPerson)
	env.RegisterActivity(fake)
	signals(env)
	env.ExecuteWorkflow(OnboardPerson, PersonInput{
		OnboardingID: "22222222-2222-4222-8222-222222222222", LegalName: "Naledi Mokoena", NationalID: "9001015009086",
		Country: "ZA", Actor: "applicant", ApplicantSubject: "user_01",
	})
	var st Status
	if env.IsWorkflowCompleted() && env.GetWorkflowError() == nil {
		if err := env.GetWorkflowResult(&st); err != nil {
			t.Fatal(err)
		}
	}
	return st, env.GetWorkflowError()
}

func verdict(env *testsuite.TestWorkflowEnvironment, after time.Duration, subject, v string) {
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(SignalKYCResult, KYCSignal{SubjectPartyID: subject, Verdict: v})
	}, after)
}

func TestAnApprovedPersonGetsAPersonalAccount(t *testing.T) {
	fake := &fakeActivities{calls: map[string]int{}}
	st, err := runPerson(t, fake, func(env *testsuite.TestWorkflowEnvironment) { verdict(env, time.Minute, "person-9", "APPROVED") })
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"prospects_created", "consents_recorded", "verifications_started", "kyc_approved", "screened", "accounts_created", "activated"}
	if st.State != StateCompleted || st.Kind != "person" || len(st.AccountIDs) != 1 || st.CompanyPartyID != "" ||
		len(st.PersonPartyIDs) != 1 || st.PersonPartyIDs[0] != "person-9" || st.Tier != "standard" {
		t.Fatalf("status = %+v", st)
	}
	for i := range want {
		if i >= len(st.Steps) || st.Steps[i] != want[i] {
			t.Fatalf("steps = %v, want %v", st.Steps, want)
		}
	}
	if fake.calls["EvaluateSector"] != 0 || fake.calls["CreateProspects"] != 0 {
		t.Fatalf("a person has no sector and no company: %v", fake.calls)
	}
}

func TestADeclinedPersonGetsNoAccountAndIsClosed(t *testing.T) {
	fake := &fakeActivities{calls: map[string]int{}}
	st, err := runPerson(t, fake, func(env *testsuite.TestWorkflowEnvironment) { verdict(env, time.Minute, "person-9", "DECLINED") })
	if err != nil {
		t.Fatal(err)
	}
	if st.State != StateDeclined || fake.calls["CreatePersonAccount"] != 0 || fake.calls["CloseParties"] != 1 {
		t.Fatalf("status = %+v, calls = %v", st, fake.calls)
	}
}

func TestAScreeningMatchDeclinesAPerson(t *testing.T) {
	fake := &fakeActivities{hits: []string{"Naledi Mokoena"}, calls: map[string]int{}}
	st, err := runPerson(t, fake, func(env *testsuite.TestWorkflowEnvironment) { verdict(env, time.Minute, "person-9", "APPROVED") })
	if err != nil || st.State != StateDeclined || st.DeclineReason != "screening_match" || fake.calls["CreatePersonAccount"] != 0 {
		t.Fatalf("status = %+v, err = %v, calls = %v", st, err, fake.calls)
	}
}

// A provider that cannot decide puts the person in the review queue; the reviewer's approval completes it.
func TestAPersonInReviewIsApprovedByAReviewer(t *testing.T) {
	fake := &fakeActivities{calls: map[string]int{}}
	st, err := runPerson(t, fake, func(env *testsuite.TestWorkflowEnvironment) {
		verdict(env, time.Minute, "person-9", "REVIEW")
		env.RegisterDelayedCallback(func() {
			env.SignalWorkflow(SignalKYCReview, KYCReviewSignal{SubjectPartyID: "person-9", Decision: "APPROVED", Reviewer: "Naledi", Note: "documents checked"})
		}, 2*time.Minute)
	})
	if err != nil || st.State != StateCompleted || fake.calls["OpenKYCReview"] != 1 {
		t.Fatalf("status = %+v, err = %v, calls = %v", st, err, fake.calls)
	}
}

func TestAPersonWithNoVerdictTimesOut(t *testing.T) {
	fake := &fakeActivities{calls: map[string]int{}}
	st, err := runPerson(t, fake, func(*testsuite.TestWorkflowEnvironment) {})
	if err != nil || st.State != StateDeclined || st.DeclineReason != "kyc_timeout" {
		t.Fatalf("status = %+v, err = %v", st, err)
	}
}
