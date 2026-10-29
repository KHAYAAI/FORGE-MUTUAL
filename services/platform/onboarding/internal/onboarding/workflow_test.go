package onboarding

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.temporal.io/sdk/testsuite"
)

// fakeActivities implements the same activity names as Activities, so the workflow runs unchanged
// against in-memory behaviour. Each test sets the fields it needs.
type fakeActivities struct {
	hits          []string
	screenErr     error
	sectorAllowed bool
	accountErr    error

	calls map[string]int
}

func (f *fakeActivities) note(name string) { f.calls[name]++ }

func (f *fakeActivities) CreateProspects(_ context.Context, _ Input) (ProspectsResult, error) {
	f.note("CreateProspects")
	return ProspectsResult{CompanyID: "company-1", PersonIDs: []string{"person-1", "person-2"}}, nil
}

func (f *fakeActivities) CreateRelationships(_ context.Context, _ Input, _ ProspectsResult) error {
	f.note("CreateRelationships")
	return nil
}

func (f *fakeActivities) RecordConsents(_ context.Context, _ Input, _ ProspectsResult) error {
	f.note("RecordConsents")
	return nil
}

func (f *fakeActivities) StartVerifications(_ context.Context, _ Input, _ ProspectsResult) error {
	f.note("StartVerifications")
	return nil
}

func (f *fakeActivities) Screen(_ context.Context, _ Input) ([]string, error) {
	f.note("Screen")
	return f.hits, f.screenErr
}

func (f *fakeActivities) EvaluateSector(_ context.Context, _ Input) (SectorResult, error) {
	f.note("EvaluateSector")
	if f.sectorAllowed {
		return SectorResult{Allowed: true, Reason: "allowed"}, nil
	}
	return SectorResult{Allowed: false, Reason: "sector_not_eligible"}, nil
}

func (f *fakeActivities) SetTier(_ context.Context, _ string, _ string) error {
	f.note("SetTier")
	return nil
}

func (f *fakeActivities) CreateAccounts(_ context.Context, _ Input, _ string) ([]string, error) {
	f.note("CreateAccounts")
	if f.accountErr != nil {
		return nil, f.accountErr
	}
	return []string{"account-1"}, nil
}

func (f *fakeActivities) ActivateParties(_ context.Context, _ []string) error {
	f.note("ActivateParties")
	return nil
}

func (f *fakeActivities) CloseParties(_ context.Context, _ []string) error {
	f.note("CloseParties")
	return nil
}

func input() Input {
	return Input{
		OnboardingID: "11111111-1111-4111-8111-111111111111", LegalName: "Forge Pilot (Pty) Ltd",
		RegistrationNumber: "2026/1", Country: "ZA", SectorCode: "software_saas", Actor: "founder",
		Directors: []DirectorInput{{LegalName: "Thandi Dube", NationalID: "8001010012345", OwnershipBps: 6000, IsUBO: true}, {LegalName: "Sipho Ndlovu", NationalID: "8002020012345"}},
	}
}

// run executes the workflow with the given KYC verdicts delivered as signals.
func run(t *testing.T, fake *fakeActivities, verdicts map[string]string) (Status, error, *testsuite.TestWorkflowEnvironment) {
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
	env.ExecuteWorkflow(OnboardBusiness, input())
	var st Status
	if env.IsWorkflowCompleted() && env.GetWorkflowError() == nil {
		if err := env.GetWorkflowResult(&st); err != nil {
			t.Fatal(err)
		}
	}
	return st, env.GetWorkflowError(), env
}

func TestApprovedApplicationCompletes(t *testing.T) {
	fake := &fakeActivities{sectorAllowed: true, calls: map[string]int{}}
	st, err, _ := run(t, fake, map[string]string{"company-1": "APPROVED", "person-1": "APPROVED", "person-2": "APPROVED"})
	if err != nil {
		t.Fatalf("workflow error: %v", err)
	}
	if st.State != StateCompleted || st.Tier != "enhanced" || len(st.AccountIDs) != 1 {
		t.Fatalf("status = %+v", st)
	}
	want := []string{"prospects_created", "relationships_created", "consents_recorded", "verifications_started", "kyc_approved", "screened", "sector_eligible", "accounts_created", "activated"}
	if len(st.Steps) != len(want) {
		t.Fatalf("steps = %v", st.Steps)
	}
	for i := range want {
		if st.Steps[i] != want[i] {
			t.Fatalf("step %d = %s, want %s", i, st.Steps[i], want[i])
		}
	}
	if fake.calls["CloseParties"] != 0 {
		t.Fatal("an approved application must not close parties")
	}
}

func TestKYCDeclineCreatesNoAccount(t *testing.T) {
	fake := &fakeActivities{sectorAllowed: true, calls: map[string]int{}}
	st, err, _ := run(t, fake, map[string]string{"company-1": "APPROVED", "person-1": "DECLINED"})
	if err != nil {
		t.Fatal(err)
	}
	if st.State != StateDeclined || st.DeclineReason != "kyc_declined:person-1" {
		t.Fatalf("status = %+v", st)
	}
	if fake.calls["CreateAccounts"] != 0 {
		t.Fatal("a declined application must not create a ledger account")
	}
	if fake.calls["CloseParties"] != 1 {
		t.Fatal("a declined application must close its parties")
	}
}

func TestScreeningMatchDeclines(t *testing.T) {
	fake := &fakeActivities{hits: []string{"Thandi Dube"}, sectorAllowed: true, calls: map[string]int{}}
	st, err, _ := run(t, fake, map[string]string{"company-1": "APPROVED", "person-1": "APPROVED", "person-2": "APPROVED"})
	if err != nil {
		t.Fatal(err)
	}
	if st.State != StateDeclined || st.DeclineReason != "screening_match" || fake.calls["CreateAccounts"] != 0 {
		t.Fatalf("status = %+v, calls = %v", st, fake.calls)
	}
}

func TestUnavailableScreeningFailsClosed(t *testing.T) {
	fake := &fakeActivities{screenErr: errors.New("list is stale"), sectorAllowed: true, calls: map[string]int{}}
	st, err, _ := run(t, fake, map[string]string{"company-1": "APPROVED", "person-1": "APPROVED", "person-2": "APPROVED"})
	if err != nil {
		t.Fatal(err)
	}
	if st.State != StateDeclined || st.DeclineReason != "screening_unavailable" || fake.calls["CreateAccounts"] != 0 {
		t.Fatalf("status = %+v", st)
	}
}

func TestSectorRefusalDeclines(t *testing.T) {
	fake := &fakeActivities{sectorAllowed: false, calls: map[string]int{}}
	st, err, _ := run(t, fake, map[string]string{"company-1": "APPROVED", "person-1": "APPROVED", "person-2": "APPROVED"})
	if err != nil {
		t.Fatal(err)
	}
	if st.State != StateDeclined || st.DeclineReason != "sector_not_eligible:sector_not_eligible" || fake.calls["CreateAccounts"] != 0 {
		t.Fatalf("status = %+v", st)
	}
}

func TestMissingVerdictTimesOut(t *testing.T) {
	fake := &fakeActivities{sectorAllowed: true, calls: map[string]int{}}
	// person-2 never answers; the test environment skips the seven-day timer.
	st, err, _ := run(t, fake, map[string]string{"company-1": "APPROVED", "person-1": "APPROVED"})
	if err != nil {
		t.Fatal(err)
	}
	if st.State != StateDeclined || st.DeclineReason != "kyc_timeout" {
		t.Fatalf("status = %+v", st)
	}
}

func TestLedgerFailureMarksTheRunFailed(t *testing.T) {
	fake := &fakeActivities{sectorAllowed: true, accountErr: errors.New("ledger down"), calls: map[string]int{}}
	_, err, _ := run(t, fake, map[string]string{"company-1": "APPROVED", "person-1": "APPROVED", "person-2": "APPROVED"})
	if err == nil {
		t.Fatal("a failed ledger call must fail the run, not complete it")
	}
}
