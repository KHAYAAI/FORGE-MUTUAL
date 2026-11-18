package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"

	paymentsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/payments/v1"
	"github.com/khayaai/forge-mutual/services/bank/payments-orchestrator/internal/payments"
)

// fakeTemporal keeps started workflows in memory and refuses a second start of the same ID, as the server does
// with REJECT_DUPLICATE.
type fakeTemporal struct {
	started map[string]payments.Input
	states  map[string]payments.Status
	signals []string
}

func newFake() *fakeTemporal {
	return &fakeTemporal{started: map[string]payments.Input{}, states: map[string]payments.Status{}}
}

func (f *fakeTemporal) ExecuteWorkflow(_ context.Context, opts client.StartWorkflowOptions, _ interface{}, args ...interface{}) (client.WorkflowRun, error) {
	if _, ok := f.started[opts.ID]; ok {
		return nil, serviceerror.NewWorkflowExecutionAlreadyStarted("already started", "req", "run")
	}
	in := args[0].(payments.Input)
	f.started[opts.ID] = in
	f.states[opts.ID] = payments.Status{PaymentID: in.PaymentID, RequestHash: in.RequestHash, State: payments.StateRunning,
		InitiatedBy: in.Actor, AmountMinor: in.AmountMinor, Asset: in.Asset}
	return nil, nil
}

func (f *fakeTemporal) SignalWorkflow(_ context.Context, id, _, name string, _ interface{}) error {
	if _, ok := f.started[id]; !ok {
		return serviceerror.NewNotFound("no such workflow")
	}
	f.signals = append(f.signals, name)
	return nil
}

func (f *fakeTemporal) QueryWorkflow(_ context.Context, id, _, _ string, _ ...interface{}) (converter.EncodedValue, error) {
	st, ok := f.states[id]
	if !ok {
		return nil, serviceerror.NewNotFound("no such workflow")
	}
	return jsonValue{st}, nil
}

type jsonValue struct{ v interface{} }

func (j jsonValue) HasValue() bool { return true }
func (j jsonValue) Get(p interface{}) error {
	b, err := json.Marshal(j.v)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, p)
}

func request(key string, mut func(*paymentsv1.CreatePaymentRequest)) *connect.Request[paymentsv1.CreatePaymentRequest] {
	m := &paymentsv1.CreatePaymentRequest{
		IdempotencyKey: key, PartyId: "11111111-1111-4111-8111-111111111111",
		FromAccountId: "0123456789abcdef0123456789abcdef", AmountMinor: "15000", Asset: "ZAR",
		BeneficiaryName: "Sipho Ndlovu", AccountNumber: "1234567890", BankCode: "250655", Actor: "founder",
	}
	if mut != nil {
		mut(m)
	}
	return connect.NewRequest(m)
}

func TestCreateIsIdempotentAndRefusesChangedRequests(t *testing.T) {
	s := New(newFake(), Config{TaskQueue: "q"})
	first, err := s.CreatePayment(context.Background(), request("p1", nil))
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.CreatePayment(context.Background(), request("p1", nil))
	if err != nil || again.Msg.PaymentId != first.Msg.PaymentId {
		t.Fatalf("replay = %v, %v", again, err)
	}
	_, err = s.CreatePayment(context.Background(), request("p1", func(m *paymentsv1.CreatePaymentRequest) { m.AmountMinor = "15001" }))
	if connect.CodeOf(err) != connect.CodeAlreadyExists {
		t.Fatalf("changed amount under the same key: %v", err)
	}
}

func TestCreateValidation(t *testing.T) {
	s := New(newFake(), Config{TaskQueue: "q"})
	cases := map[string]func(*paymentsv1.CreatePaymentRequest){
		"zero amount":     func(m *paymentsv1.CreatePaymentRequest) { m.AmountMinor = "0" },
		"decimal amount":  func(m *paymentsv1.CreatePaymentRequest) { m.AmountMinor = "150.00" },
		"over the cap":    func(m *paymentsv1.CreatePaymentRequest) { m.AmountMinor = "1000000001" },
		"dollars":         func(m *paymentsv1.CreatePaymentRequest) { m.Asset = "USD" },
		"bad account":     func(m *paymentsv1.CreatePaymentRequest) { m.AccountNumber = "12ab" },
		"bad ledger acct": func(m *paymentsv1.CreatePaymentRequest) { m.FromAccountId = "not-hex" },
		"no actor":        func(m *paymentsv1.CreatePaymentRequest) { m.Actor = "" },
		"long reference":  func(m *paymentsv1.CreatePaymentRequest) { m.Reference = strings.Repeat("x", 141) },
	}
	for name, mut := range cases {
		_, err := s.CreatePayment(context.Background(), request("v-"+strings.ReplaceAll(name, " ", "-"), mut))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: want InvalidArgument, got %v", name, err)
		}
	}
}

func TestSimulationScenariosAreRefusedUnlessEnabled(t *testing.T) {
	scenario := func(m *paymentsv1.CreatePaymentRequest) { m.SimulationScenario = "stall" }
	if _, err := New(newFake(), Config{TaskQueue: "q"}).CreatePayment(context.Background(), request("s1", scenario)); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("scenario on a production service: %v", err)
	}
	if _, err := New(newFake(), Config{TaskQueue: "q", AllowSimulation: true}).CreatePayment(context.Background(), request("s2", scenario)); err != nil {
		t.Fatalf("scenario with simulation on: %v", err)
	}
}

func TestInitiatorCannotApproveTheirOwnPayment(t *testing.T) {
	tc := newFake()
	s := New(tc, Config{TaskQueue: "q"})
	created, _ := s.CreatePayment(context.Background(), request("a1", nil))
	_, err := s.ApprovePayment(context.Background(), connect.NewRequest(&paymentsv1.ApprovePaymentRequest{
		PaymentId: created.Msg.PaymentId, Approve: true, Approver: "founder",
	}))
	if connect.CodeOf(err) != connect.CodePermissionDenied || len(tc.signals) != 0 {
		t.Fatalf("self-approval: %v, signals=%v", err, tc.signals)
	}
	if _, err := s.ApprovePayment(context.Background(), connect.NewRequest(&paymentsv1.ApprovePaymentRequest{
		PaymentId: created.Msg.PaymentId, Approve: true, Approver: "checker",
	})); err != nil || len(tc.signals) != 1 {
		t.Fatalf("checker approval: %v, signals=%v", err, tc.signals)
	}
}

func TestRepairOnlyForPaymentsAwaitingIt(t *testing.T) {
	s := New(newFake(), Config{TaskQueue: "q"})
	created, _ := s.CreatePayment(context.Background(), request("r1", nil))
	_, err := s.ResolveRepair(context.Background(), connect.NewRequest(&paymentsv1.ResolveRepairRequest{
		PaymentId: created.Msg.PaymentId, Settled: true, Operator: "ops",
	}))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("repair of a running payment: %v", err)
	}
}

func TestGetPaymentAndUnknown(t *testing.T) {
	s := New(newFake(), Config{TaskQueue: "q"})
	created, _ := s.CreatePayment(context.Background(), request("g1", nil))
	got, err := s.GetPayment(context.Background(), connect.NewRequest(&paymentsv1.GetPaymentRequest{PaymentId: created.Msg.PaymentId}))
	if err != nil || got.Msg.Status.State != paymentsv1.PaymentState_PAYMENT_STATE_RUNNING {
		t.Fatalf("get = %v, %v", got, err)
	}
	_, err = s.GetPayment(context.Background(), connect.NewRequest(&paymentsv1.GetPaymentRequest{PaymentId: "11111111-1111-4111-8111-111111111111"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("unknown payment: %v", err)
	}
}

func TestEveryWorkflowStateHasAProtoName(t *testing.T) {
	for _, s := range []string{payments.StateRunning, payments.StateAwaitingApproval, payments.StateReserved,
		payments.StateCompleted, payments.StateDeclined, payments.StateFailed, payments.StateCancelled, payments.StateRepairRequired} {
		if stateOf(s) == paymentsv1.PaymentState_PAYMENT_STATE_UNSPECIFIED {
			t.Errorf("state %s has no protobuf value", s)
		}
	}
}
