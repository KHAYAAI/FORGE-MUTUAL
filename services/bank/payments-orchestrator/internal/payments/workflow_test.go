package payments

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"go.temporal.io/sdk/testsuite"

	railsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/rails/v1"
)

// fakeAct implements the activity names of Activities in memory. Each test sets the outcomes it needs.
type fakeAct struct {
	mu sync.Mutex

	partyStatus string
	hits        []string
	assetDeny   bool
	limits      Decision
	reserve     ReserveResult
	reserveErr  error
	initiate    RailResult
	initiateErr error
	polls       []railsv1.RailStatus // one per status poll; the last value repeats
	pollIdx     int
	settleErr   error

	calls    map[string]int
	released []string // reasons passed to Release
	states   []string // states passed to Record
}

func newFake() *fakeAct {
	return &fakeAct{
		partyStatus: "PARTY_STATUS_ACTIVE",
		limits:      Decision{Allow: true, Reason: "ok"},
		reserve:     ReserveResult{OK: true, TransferID: "hold-1"},
		initiate:    RailResult{Status: railsv1.RailStatus_RAIL_STATUS_ACCEPTED, Reference: "RS-1"},
		polls:       []railsv1.RailStatus{railsv1.RailStatus_RAIL_STATUS_PENDING, railsv1.RailStatus_RAIL_STATUS_SETTLED},
		calls:       map[string]int{},
	}
}

func (f *fakeAct) note(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls[name]++
}

func (f *fakeAct) count(name string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[name]
}

func (f *fakeAct) LoadParty(_ context.Context, _ string) (PartyInfo, error) {
	f.note("LoadParty")
	return PartyInfo{Status: f.partyStatus, Tier: "standard"}, nil
}

func (f *fakeAct) Screen(_ context.Context, _ Input) ([]string, error) {
	f.note("Screen")
	return f.hits, nil
}

func (f *fakeAct) CheckAssetEntity(_ context.Context, _ Input) (Decision, error) {
	f.note("CheckAssetEntity")
	if f.assetDeny {
		return Decision{Reason: "asset_default_deny"}, nil
	}
	return Decision{Allow: true, Reason: "ok"}, nil
}

func (f *fakeAct) CheckLimits(_ context.Context, _ Input, _ PartyInfo) (Decision, error) {
	f.note("CheckLimits")
	return f.limits, nil
}

func (f *fakeAct) Reserve(_ context.Context, _ Input) (ReserveResult, error) {
	f.note("Reserve")
	return f.reserve, f.reserveErr
}

func (f *fakeAct) RailInitiate(_ context.Context, _ Input) (RailResult, error) {
	f.note("RailInitiate")
	return f.initiate, f.initiateErr
}

func (f *fakeAct) RailStatus(_ context.Context, _ Input) (RailResult, error) {
	f.note("RailStatus")
	f.mu.Lock()
	defer f.mu.Unlock()
	i := f.pollIdx
	if i >= len(f.polls) {
		i = len(f.polls) - 1
	}
	f.pollIdx++
	return RailResult{Status: f.polls[i], Reference: "RS-1"}, nil
}

func (f *fakeAct) Settle(_ context.Context, _ Input, _, _ string) error {
	f.note("Settle")
	return f.settleErr
}

func (f *fakeAct) Release(_ context.Context, _ Input, _, _, reason, _ string) error {
	f.note("Release")
	f.mu.Lock()
	defer f.mu.Unlock()
	f.released = append(f.released, reason)
	return nil
}

func (f *fakeAct) Record(_ context.Context, _ Input, state, _, _ string) error {
	f.note("Record")
	f.mu.Lock()
	defer f.mu.Unlock()
	f.states = append(f.states, state)
	return nil
}

func input() Input {
	return Input{
		PaymentID: "22222222-2222-4222-8222-222222222222", RequestHash: "h",
		PartyID: "11111111-1111-4111-8111-111111111111", FromAccountID: "0123456789abcdef0123456789abcdef",
		AmountMinor: "15000", Asset: "ZAR", BeneficiaryName: "Sipho Ndlovu", AccountNumber: "1234567890",
		BankCode: "250655", Actor: "initiator",
	}
}

// run executes the payment with the given signals delivered at fixed virtual times.
func run(t *testing.T, fake *fakeAct, in Input, signals ...func(env *testsuite.TestWorkflowEnvironment)) (Status, error) {
	t.Helper()
	var ts testsuite.WorkflowTestSuite
	env := ts.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(DomesticPayment)
	env.RegisterActivity(fake)
	for _, s := range signals {
		s(env)
	}
	env.ExecuteWorkflow(DomesticPayment, in)
	if err := env.GetWorkflowError(); err != nil {
		return Status{}, err
	}
	var st Status
	if err := env.GetWorkflowResult(&st); err != nil {
		t.Fatal(err)
	}
	return st, nil
}

func signalAt(d time.Duration, name string, arg interface{}) func(env *testsuite.TestWorkflowEnvironment) {
	return func(env *testsuite.TestWorkflowEnvironment) {
		env.RegisterDelayedCallback(func() { env.SignalWorkflow(name, arg) }, d)
	}
}

func TestSettledPaymentCompletes(t *testing.T) {
	fake := newFake()
	st, err := run(t, fake, input())
	if err != nil {
		t.Fatal(err)
	}
	if st.State != StateCompleted || fake.count("Settle") != 1 || fake.count("Release") != 0 {
		t.Fatalf("status = %+v, settle=%d release=%d", st, fake.count("Settle"), fake.count("Release"))
	}
}

func TestRailRejectionReleasesTheHold(t *testing.T) {
	fake := newFake()
	fake.polls = []railsv1.RailStatus{railsv1.RailStatus_RAIL_STATUS_REJECTED}
	st, err := run(t, fake, input())
	if err != nil {
		t.Fatal(err)
	}
	if st.State != StateFailed || st.Reason != "rail_rejected" || fake.count("Settle") != 0 {
		t.Fatalf("status = %+v", st)
	}
	if len(fake.released) != 1 || fake.released[0] != "rail_rejected" {
		t.Fatalf("released = %v", fake.released)
	}
}

func TestRailReturnReleasesTheHold(t *testing.T) {
	fake := newFake()
	fake.polls = []railsv1.RailStatus{railsv1.RailStatus_RAIL_STATUS_RETURNED}
	st, _ := run(t, fake, input())
	if st.State != StateFailed || st.Reason != "rail_returned" || fake.count("Settle") != 0 {
		t.Fatalf("status = %+v", st)
	}
}

func TestRejectionAtInitiationReleasesTheHold(t *testing.T) {
	fake := newFake()
	fake.initiate = RailResult{Status: railsv1.RailStatus_RAIL_STATUS_REJECTED}
	st, _ := run(t, fake, input())
	if st.State != StateFailed || st.Reason != "rail_rejected_at_initiation" || fake.count("RailStatus") != 0 {
		t.Fatalf("status = %+v", st)
	}
	if fake.count("Release") != 1 {
		t.Fatal("the hold must be released")
	}
}

func TestDeclinesBeforeAnyHold(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(f *fakeAct)
		reason string
	}{
		{"party not active", func(f *fakeAct) { f.partyStatus = "PARTY_STATUS_FROZEN" }, "party_not_active"},
		{"beneficiary match", func(f *fakeAct) { f.hits = []string{"Sipho Ndlovu"} }, "beneficiary_screening_match"},
		{"asset or entity refused", func(f *fakeAct) { f.assetDeny = true }, "policy:asset_default_deny"},
		{"limit exceeded", func(f *fakeAct) { f.limits = Decision{Reason: "default_deny"} }, "limits:default_deny"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := newFake()
			tc.mutate(fake)
			st, err := run(t, fake, input())
			if err != nil {
				t.Fatal(err)
			}
			if st.State != StateDeclined || st.Reason != tc.reason {
				t.Fatalf("status = %+v", st)
			}
			if fake.count("Reserve") != 0 {
				t.Fatal("a declined payment must not place a hold")
			}
		})
	}
}

func TestInsufficientFundsDeclines(t *testing.T) {
	fake := newFake()
	fake.reserve = ReserveResult{OK: false, Reason: "insufficient_funds"}
	st, _ := run(t, fake, input())
	if st.State != StateDeclined || st.Reason != "insufficient_funds" || fake.count("RailInitiate") != 0 {
		t.Fatalf("status = %+v", st)
	}
}

func TestStepUpApprovedByAChecker(t *testing.T) {
	fake := newFake()
	fake.limits = Decision{Allow: true, StepUp: true, Reason: "above_stepup"}
	st, err := run(t, fake, input(), signalAt(time.Minute, SignalApproval, Approval{Approve: true, Approver: "checker"}))
	if err != nil {
		t.Fatal(err)
	}
	if st.State != StateCompleted {
		t.Fatalf("status = %+v", st)
	}
}

func TestInitiatorCannotApproveTheirOwnPayment(t *testing.T) {
	fake := newFake()
	fake.limits = Decision{Allow: true, StepUp: true, Reason: "above_stepup"}
	st, err := run(t, fake, input(), signalAt(time.Minute, SignalApproval, Approval{Approve: true, Approver: "initiator"}))
	if err != nil {
		t.Fatal(err)
	}
	if st.State != StateCancelled || st.Reason != "approval_timeout" || fake.count("Reserve") != 0 {
		t.Fatalf("self-approval must be ignored and the payment must lapse: %+v", st)
	}
}

func TestRefusedApprovalCancelsBeforeAnyHold(t *testing.T) {
	fake := newFake()
	fake.limits = Decision{Allow: true, StepUp: true, Reason: "above_stepup"}
	st, _ := run(t, fake, input(), signalAt(time.Minute, SignalApproval, Approval{Approve: false, Approver: "checker"}))
	if st.State != StateCancelled || st.Reason != "approval_refused" || fake.count("Reserve") != 0 {
		t.Fatalf("status = %+v", st)
	}
}

func TestUnknownOutcomeGoesToRepairAndIsNotReleased(t *testing.T) {
	fake := newFake()
	fake.polls = []railsv1.RailStatus{railsv1.RailStatus_RAIL_STATUS_PENDING} // never final
	in := input()
	in.MaxPolls = 3
	st, err := run(t, fake, in, signalAt(2*time.Hour, SignalRepair, Repair{Settled: true, Operator: "ops"}))
	if err != nil {
		t.Fatal(err)
	}
	if st.State != StateCompleted || fake.count("Settle") != 1 || fake.count("Release") != 0 {
		t.Fatalf("an operator-confirmed settlement must post the hold: %+v settle=%d release=%d", st, fake.count("Settle"), fake.count("Release"))
	}
}

func TestOperatorCanRefuseARepairedPayment(t *testing.T) {
	fake := newFake()
	fake.polls = []railsv1.RailStatus{railsv1.RailStatus_RAIL_STATUS_PENDING}
	in := input()
	in.MaxPolls = 2
	st, _ := run(t, fake, in, signalAt(2*time.Hour, SignalRepair, Repair{Settled: false, Operator: "ops"}))
	if st.State != StateFailed || st.Reason != "repair_not_settled" || fake.count("Release") != 1 {
		t.Fatalf("status = %+v", st)
	}
}

func TestUnknownInitiationGoesToRepair(t *testing.T) {
	fake := newFake()
	fake.initiateErr = errors.New("rail unreachable after retries")
	st, _ := run(t, fake, input(), signalAt(time.Hour, SignalRepair, Repair{Settled: false, Operator: "ops"}))
	if st.State != StateFailed || st.Reason != "repair_not_settled" || fake.count("Release") != 1 {
		t.Fatalf("an initiation whose outcome is unknown must not auto-release before a human decides: %+v", st)
	}
}

func TestSettlementFailureAfterRailSettledGoesToRepair(t *testing.T) {
	fake := newFake()
	fake.settleErr = errors.New("ledger down")
	st, _ := run(t, fake, input(), signalAt(time.Hour, SignalRepair, Repair{Settled: true, Operator: "ops"}))
	if st.State != StateCompleted && st.State != StateRepairRequired {
		t.Fatalf("status = %+v", st)
	}
	if fake.count("Release") != 0 {
		t.Fatal("money the rail has settled must never be released")
	}
}

func TestDefaultsAreFilledInTheInput(t *testing.T) {
	in := Input{}.WithDefaults()
	if in.PollSeconds != DefaultPollSeconds || in.MaxPolls != DefaultMaxPolls || in.ReserveTimeoutSeconds != DefaultReserveSeconds || in.ApprovalSeconds != DefaultApprovalSecs {
		t.Fatalf("defaults = %+v", in)
	}
	if DefaultReserveSeconds > 7*24*60*60 {
		t.Fatal("a hold may not outlive the ledger's seven-day pending limit")
	}
}
