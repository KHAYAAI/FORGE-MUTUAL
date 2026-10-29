package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"

	onboardingv1 "github.com/khayaai/forge-mutual/pkg/api/forge/onboarding/v1"
	"github.com/khayaai/forge-mutual/services/platform/onboarding/internal/kyc"
	"github.com/khayaai/forge-mutual/services/platform/onboarding/internal/onboarding"
)

// fakeTemporal records starts and signals in memory. A second start of the same ID is refused as
// already started, as the Temporal server does with REJECT_DUPLICATE.
type fakeTemporal struct {
	started   map[string]onboarding.Input
	signals   []onboarding.KYCSignal
	signalErr error
}

func newFakeTemporal() *fakeTemporal { return &fakeTemporal{started: map[string]onboarding.Input{}} }

func (f *fakeTemporal) ExecuteWorkflow(_ context.Context, opts client.StartWorkflowOptions, _ interface{}, args ...interface{}) (client.WorkflowRun, error) {
	if _, ok := f.started[opts.ID]; ok {
		return nil, serviceerror.NewWorkflowExecutionAlreadyStarted("already started", "req", "run")
	}
	f.started[opts.ID] = args[0].(onboarding.Input)
	return nil, nil
}

func (f *fakeTemporal) SignalWorkflow(_ context.Context, workflowID, _, _ string, arg interface{}) error {
	if f.signalErr != nil {
		return f.signalErr
	}
	if _, ok := f.started[workflowID]; !ok {
		return serviceerror.NewNotFound("workflow not found")
	}
	f.signals = append(f.signals, arg.(onboarding.KYCSignal))
	return nil
}

func (f *fakeTemporal) QueryWorkflow(_ context.Context, workflowID, _, _ string, _ ...interface{}) (converter.EncodedValue, error) {
	in, ok := f.started[workflowID]
	if !ok {
		return nil, serviceerror.NewNotFound("workflow not found")
	}
	st := onboarding.Status{OnboardingID: workflowID, RequestHash: in.RequestHash, State: onboarding.StateRunning, Steps: []string{"prospects_created"}}
	return jsonValue{st}, nil
}

// jsonValue is a minimal converter.EncodedValue for tests.
type jsonValue struct{ v interface{} }

func (j jsonValue) HasValue() bool { return true }
func (j jsonValue) Get(p interface{}) error {
	b, err := json.Marshal(j.v)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, p)
}

func request(key string, mut func(*onboardingv1.StartBusinessOnboardingRequest)) *connect.Request[onboardingv1.StartBusinessOnboardingRequest] {
	m := &onboardingv1.StartBusinessOnboardingRequest{
		IdempotencyKey: key, LegalName: "Forge Pilot (Pty) Ltd", RegistrationNumber: "2026/9", Country: "ZA",
		SectorCode: "software_saas", Actor: "founder",
		Directors: []*onboardingv1.Director{{LegalName: "Thandi Dube", NationalId: "8001010012345", OwnershipBps: 6000, IsUbo: true}},
	}
	if mut != nil {
		mut(m)
	}
	return connect.NewRequest(m)
}

func TestStartIsIdempotentByKey(t *testing.T) {
	tc := newFakeTemporal()
	s := New(tc, mustFake(t), "onboarding")
	first, err := s.StartBusinessOnboarding(context.Background(), request("k1", nil))
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.StartBusinessOnboarding(context.Background(), request("k1", nil))
	if err != nil || again.Msg.OnboardingId != first.Msg.OnboardingId {
		t.Fatalf("same request must return the same onboarding: %v %v", again, err)
	}
	if len(tc.started) != 1 {
		t.Fatalf("workflows started = %d, want 1", len(tc.started))
	}
}

func TestSameKeyDifferentRequestIsRefused(t *testing.T) {
	s := New(newFakeTemporal(), mustFake(t), "onboarding")
	if _, err := s.StartBusinessOnboarding(context.Background(), request("k1", nil)); err != nil {
		t.Fatal(err)
	}
	_, err := s.StartBusinessOnboarding(context.Background(), request("k1", func(m *onboardingv1.StartBusinessOnboardingRequest) {
		m.LegalName = "Someone Else (Pty) Ltd"
	}))
	if connect.CodeOf(err) != connect.CodeAlreadyExists {
		t.Fatalf("want AlreadyExists, got %v", err)
	}
}

func TestValidationRejectsBadApplications(t *testing.T) {
	s := New(newFakeTemporal(), mustFake(t), "onboarding")
	cases := map[string]func(*onboardingv1.StartBusinessOnboardingRequest){
		"no directors":     func(m *onboardingv1.StartBusinessOnboardingRequest) { m.Directors = nil },
		"bad national id":  func(m *onboardingv1.StartBusinessOnboardingRequest) { m.Directors[0].NationalId = "12" },
		"bad country":      func(m *onboardingv1.StartBusinessOnboardingRequest) { m.Country = "zaf" },
		"ownership over 1": func(m *onboardingv1.StartBusinessOnboardingRequest) { m.Directors[0].OwnershipBps = 10001 },
		"no sector":        func(m *onboardingv1.StartBusinessOnboardingRequest) { m.SectorCode = "" },
		"no registration":  func(m *onboardingv1.StartBusinessOnboardingRequest) { m.RegistrationNumber = " " },
	}
	for name, mut := range cases {
		_, err := s.StartBusinessOnboarding(context.Background(), request("v-"+name, mut))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: want InvalidArgument, got %v", name, err)
		}
	}
}

func TestGetOnboardingReportsState(t *testing.T) {
	tc := newFakeTemporal()
	s := New(tc, mustFake(t), "onboarding")
	start, err := s.StartBusinessOnboarding(context.Background(), request("k2", nil))
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.GetOnboarding(context.Background(), connect.NewRequest(&onboardingv1.GetOnboardingRequest{OnboardingId: start.Msg.OnboardingId}))
	if err != nil {
		t.Fatal(err)
	}
	if got.Msg.Status.State != onboardingv1.OnboardingState_ONBOARDING_STATE_RUNNING || len(got.Msg.Status.Steps) != 1 {
		t.Fatalf("status = %+v", got.Msg.Status)
	}
	_, err = s.GetOnboarding(context.Background(), connect.NewRequest(&onboardingv1.GetOnboardingRequest{OnboardingId: "11111111-1111-4111-8111-111111111111"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("unknown onboarding: %v", err)
	}
}

func mustFake(t *testing.T) kyc.Provider {
	t.Helper()
	p, err := kyc.NewFake([]byte("0123456789abcdef-test-secret"))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

const testSecret = "0123456789abcdef-test-secret"

func post(t *testing.T, h http.Handler, body, sig string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/webhooks/kyc", strings.NewReader(body))
	if sig != "" {
		r.Header.Set("X-Forge-Signature", sig)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestWebhookSignalsOnlyWhenSigned(t *testing.T) {
	tc := newFakeTemporal()
	s := New(tc, mustFake(t), "onboarding")
	start, err := s.StartBusinessOnboarding(context.Background(), request("k3", nil))
	if err != nil {
		t.Fatal(err)
	}
	id := start.Msg.OnboardingId
	body := `{"session_id":"s1","onboarding_id":"` + id + `","subject_party_id":"party-1","verdict":"APPROVED"}`
	h := s.WebhookHandler()

	if w := post(t, h, body, "bad"); w.Code != http.StatusUnauthorized {
		t.Fatalf("forged webhook: %d", w.Code)
	}
	if len(tc.signals) != 0 {
		t.Fatal("a forged webhook must not signal the workflow")
	}
	if w := post(t, h, body, kyc.Sign([]byte(testSecret), []byte(body))); w.Code != http.StatusAccepted {
		t.Fatalf("signed webhook: %d", w.Code)
	}
	if len(tc.signals) != 1 || tc.signals[0].SubjectPartyID != "party-1" || tc.signals[0].Verdict != "APPROVED" {
		t.Fatalf("signals = %+v", tc.signals)
	}
	unknown := `{"session_id":"s2","onboarding_id":"11111111-1111-4111-8111-111111111111","subject_party_id":"p","verdict":"APPROVED"}`
	if w := post(t, h, unknown, kyc.Sign([]byte(testSecret), []byte(unknown))); w.Code != http.StatusNotFound {
		t.Fatalf("unknown onboarding: %d", w.Code)
	}
	if w := post(t, h, body, ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("missing signature: %d", w.Code)
	}
}

func TestDiditNotConfiguredIsUnavailableNotApproved(t *testing.T) {
	s := New(newFakeTemporal(), kyc.Didit{}, "onboarding")
	w := post(t, s.WebhookHandler(), `{}`, "anything")
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("unconfigured provider must refuse: %d", w.Code)
	}
}

func TestTemporalFailureIsReportedAsUnavailable(t *testing.T) {
	tc := newFakeTemporal()
	tc.signalErr = errors.New("connection refused")
	s := New(tc, mustFake(t), "onboarding")
	body := `{"session_id":"s1","onboarding_id":"11111111-1111-4111-8111-111111111111","subject_party_id":"p","verdict":"APPROVED"}`
	w := post(t, s.WebhookHandler(), body, kyc.Sign([]byte(testSecret), []byte(body)))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("workflow service down: %d", w.Code)
	}
}
