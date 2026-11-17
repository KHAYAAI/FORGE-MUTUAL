package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"

	onboardingv1 "github.com/khayaai/forge-mutual/pkg/api/forge/onboarding/v1"
	"github.com/khayaai/forge-mutual/services/platform/onboarding/internal/kyc"
	"github.com/khayaai/forge-mutual/services/platform/onboarding/internal/onboarding"
	"github.com/khayaai/forge-mutual/services/platform/onboarding/internal/store"
)

// fakeTemporal records starts and signals in memory. A second start of the same ID is refused as
// already started, as the Temporal server does with REJECT_DUPLICATE.
type fakeTemporal struct {
	started       map[string]onboarding.Input
	signals       []onboarding.KYCSignal
	reviewSignals []onboarding.KYCReviewSignal
	signalErr     error
	kinds         map[string]string
}

func newFakeTemporal() *fakeTemporal {
	return &fakeTemporal{started: map[string]onboarding.Input{}, kinds: map[string]string{}}
}

func (f *fakeTemporal) ExecuteWorkflow(_ context.Context, opts client.StartWorkflowOptions, _ interface{}, args ...interface{}) (client.WorkflowRun, error) {
	if _, ok := f.started[opts.ID]; ok {
		return nil, serviceerror.NewWorkflowExecutionAlreadyStarted("already started", "req", "run")
	}
	switch in := args[0].(type) {
	case onboarding.Input:
		f.started[opts.ID] = in
	case onboarding.PersonInput:
		f.started[opts.ID] = onboarding.Input{OnboardingID: in.OnboardingID, RequestHash: in.RequestHash, LegalName: in.LegalName}
		f.kinds[opts.ID] = "person"
	}
	return nil, nil
}

func (f *fakeTemporal) SignalWorkflow(_ context.Context, workflowID, _, _ string, arg interface{}) error {
	if f.signalErr != nil {
		return f.signalErr
	}
	if _, ok := f.started[workflowID]; !ok {
		return serviceerror.NewNotFound("workflow not found")
	}
	switch v := arg.(type) {
	case onboarding.KYCSignal:
		f.signals = append(f.signals, v)
	case onboarding.KYCReviewSignal:
		f.reviewSignals = append(f.reviewSignals, v)
	}
	return nil
}

func (f *fakeTemporal) QueryWorkflow(_ context.Context, workflowID, _, _ string, _ ...interface{}) (converter.EncodedValue, error) {
	in, ok := f.started[workflowID]
	if !ok {
		return nil, serviceerror.NewNotFound("workflow not found")
	}
	st := onboarding.Status{OnboardingID: workflowID, RequestHash: in.RequestHash, State: onboarding.StateRunning, Steps: []string{"prospects_created"}, Kind: f.kinds[workflowID]}
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
	// A verdict for an onboarding that does not exist is authentic and moot. It is acknowledged, so that a provider
	// does not retry it, and it signals nothing.
	unknown := `{"session_id":"s2","onboarding_id":"11111111-1111-4111-8111-111111111111","subject_party_id":"p","verdict":"APPROVED"}`
	if w := post(t, h, unknown, kyc.Sign([]byte(testSecret), []byte(unknown))); w.Code != http.StatusOK {
		t.Fatalf("unknown onboarding: %d, want 200 (acknowledged)", w.Code)
	}
	if len(tc.signals) != 1 {
		t.Fatalf("a verdict for an unknown onboarding must not signal: %+v", tc.signals)
	}
	if w := post(t, h, body, ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("missing signature: %d", w.Code)
	}
}

const diditTestSecret = "whsec_service_test_secret_0123456789"

func mustDidit(t *testing.T, env string) *kyc.Didit {
	t.Helper()
	d, err := kyc.NewDidit(kyc.DiditConfig{
		APIKey: "k", KYCWorkflowID: "11111111-1111-4111-8111-111111111111", KYBWorkflowID: "22222222-2222-4222-8222-222222222222",
		WebhookSecret: diditTestSecret, Environment: env,
	})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// postDidit sends a delivery signed the way Didit signs it, at the current time unless the header is changed.
func postDidit(t *testing.T, h http.Handler, payload map[string]any, mod func(http.Header)) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	sig, err := kyc.SignDidit(diditTestSecret, body)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodPost, "/webhooks/kyc", strings.NewReader(string(body)))
	r.Header.Set("X-Signature-V2", sig)
	r.Header.Set("X-Timestamp", strconv.FormatInt(time.Now().Unix(), 10))
	if mod != nil {
		mod(r.Header)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func diditPayload(onboarding, subject, status string) map[string]any {
	return map[string]any{
		"event_id": "e1", "webhook_type": "status.updated", "environment": "live", "session_id": "sess-1", "status": status,
		"vendor_data": subject, "metadata": map[string]any{"onboarding_id": onboarding, "subject_party_id": subject},
		"decision": map[string]any{"status": status, "score": 98.0},
	}
}

// A Didit delivery reaches the workflow as the verdict it is, and only when it authenticates.
func TestDiditDeliveriesSignalTheWorkflowAsVerdicts(t *testing.T) {
	tc := newFakeTemporal()
	s := New(tc, mustDidit(t, "live"), "onboarding")
	start, err := s.StartBusinessOnboarding(context.Background(), request("k-didit", nil))
	if err != nil {
		t.Fatal(err)
	}
	id := start.Msg.OnboardingId
	h := s.WebhookHandler()

	for status, want := range map[string]string{"Approved": "APPROVED", "Declined": "DECLINED", "In Review": "REVIEW"} {
		tc.signals = nil
		if w := postDidit(t, h, diditPayload(id, "party-1", status), nil); w.Code != http.StatusAccepted {
			t.Fatalf("%s: %d", status, w.Code)
		}
		if len(tc.signals) != 1 || tc.signals[0].Verdict != want || tc.signals[0].SubjectPartyID != "party-1" || tc.signals[0].SessionID != "sess-1" {
			t.Fatalf("%s: signals = %+v", status, tc.signals)
		}
	}

	// Not a verdict: acknowledged, nothing signalled.
	tc.signals = nil
	for _, status := range []string{"In Progress", "Not Started", "Awaiting User", "Abandoned"} {
		if w := postDidit(t, h, diditPayload(id, "party-1", status), nil); w.Code != http.StatusOK {
			t.Errorf("%s: %d, want 200", status, w.Code)
		}
	}
	if len(tc.signals) != 0 {
		t.Fatalf("a status that is not a verdict must not signal: %+v", tc.signals)
	}
}

func TestDiditDeliveriesThatDoNotAuthenticateNeverSignal(t *testing.T) {
	tc := newFakeTemporal()
	s := New(tc, mustDidit(t, "live"), "onboarding")
	start, _ := s.StartBusinessOnboarding(context.Background(), request("k-didit2", nil))
	h := s.WebhookHandler()
	p := diditPayload(start.Msg.OnboardingId, "party-1", "Approved")

	for name, mod := range map[string]func(http.Header){
		"no signature": func(h http.Header) { h.Del("X-Signature-V2") },
		"forged":       func(h http.Header) { h.Set("X-Signature-V2", strings.Repeat("a", 64)) },
		"stale":        func(h http.Header) { h.Set("X-Timestamp", strconv.FormatInt(time.Now().Add(-time.Hour).Unix(), 10)) },
		"the fake's header only": func(h http.Header) {
			h.Del("X-Signature-V2")
			h.Set("X-Forge-Signature", "anything")
		},
	} {
		if w := postDidit(t, h, p, mod); w.Code != http.StatusUnauthorized {
			t.Errorf("%s: %d, want 401", name, w.Code)
		}
	}
	if len(tc.signals) != 0 {
		t.Fatalf("a delivery that does not authenticate must not signal: %+v", tc.signals)
	}
}

// A sandbox event at a live service is authentic but wrong: 400, not retried, and it never signals.
func TestASandboxVerdictCannotApproveALiveOnboarding(t *testing.T) {
	tc := newFakeTemporal()
	s := New(tc, mustDidit(t, "live"), "onboarding")
	start, _ := s.StartBusinessOnboarding(context.Background(), request("k-didit3", nil))
	p := diditPayload(start.Msg.OnboardingId, "party-1", "Approved")
	p["environment"] = "sandbox"
	p["sandbox_scenario"] = "approve"
	if w := postDidit(t, s.WebhookHandler(), p, nil); w.Code != http.StatusBadRequest {
		t.Fatalf("sandbox event at a live service: %d, want 400", w.Code)
	}
	if len(tc.signals) != 0 {
		t.Fatal("a sandbox verdict must never reach a live workflow")
	}
}

// A verdict for an onboarding that has ended is acknowledged with 200, not 404, so that Didit does not retry it.
func TestADiditVerdictForAnEndedOnboardingIsAcknowledged(t *testing.T) {
	s := New(newFakeTemporal(), mustDidit(t, "live"), "onboarding")
	w := postDidit(t, s.WebhookHandler(), diditPayload("11111111-1111-4111-8111-111111111111", "party-1", "Approved"), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("ended onboarding: %d, want 200", w.Code)
	}
}

// What the handler did with a delivery reaches the operator's log: why a delivery was refused, and which verdict was
// delivered. A 401 from outside cannot say whether the secret, the clock or the URL is wrong. The log must not hold the
// secret, the signature, or what the person's check found.
func TestWebhookSaysWhatItDidWithEachDelivery(t *testing.T) {
	var buf bytes.Buffer
	tc := newFakeTemporal()
	s := New(tc, mustDidit(t, "live"), "onboarding").WithLogger(slog.New(slog.NewJSONHandler(&buf, nil)))
	start, err := s.StartBusinessOnboarding(context.Background(), request("k-log", nil))
	if err != nil {
		t.Fatal(err)
	}
	id := start.Msg.OnboardingId
	h := s.WebhookHandler()

	stale := func(h http.Header) { h.Set("X-Timestamp", strconv.FormatInt(time.Now().Add(-time.Hour).Unix(), 10)) }
	if w := postDidit(t, h, diditPayload(id, "party-1", "Approved"), stale); w.Code != http.StatusUnauthorized {
		t.Fatalf("stale: %d", w.Code)
	}
	wrongEnv := diditPayload(id, "party-1", "Approved")
	wrongEnv["environment"] = "sandbox"
	if w := postDidit(t, h, wrongEnv, nil); w.Code != http.StatusBadRequest {
		t.Fatalf("wrong environment: %d", w.Code)
	}
	if w := postDidit(t, h, diditPayload(id, "party-1", "Approved"), nil); w.Code != http.StatusAccepted {
		t.Fatalf("verdict: %d", w.Code)
	}
	if w := postDidit(t, h, diditPayload(id, "party-1", "In Progress"), nil); w.Code != http.StatusOK {
		t.Fatalf("not a verdict: %d", w.Code)
	}

	var lines []map[string]any
	for _, raw := range bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n")) {
		var line map[string]any
		if err := json.Unmarshal(raw, &line); err != nil {
			t.Fatalf("log line %q: %v", raw, err)
		}
		lines = append(lines, line)
	}
	if len(lines) != 3 {
		t.Fatalf("%d log lines, want 3 (a refusal for the clock, a refusal for the environment, the verdict; a status that is not a verdict is not logged):\n%s", len(lines), buf.String())
	}
	if lines[0]["msg"] != "kyc webhook refused" || lines[0]["status"] != float64(401) || !strings.Contains(lines[0]["reason"].(string), "old, and 300s is allowed") {
		t.Errorf("the stale delivery: %v", lines[0])
	}
	if lines[1]["msg"] != "kyc webhook refused" || lines[1]["status"] != float64(400) || !strings.Contains(lines[1]["reason"].(string), `"sandbox" environment`) {
		t.Errorf("the sandbox delivery: %v", lines[1])
	}
	if lines[2]["msg"] != "kyc verdict delivered" || lines[2]["onboarding_id"] != id || lines[2]["subject_party_id"] != "party-1" ||
		lines[2]["verdict"] != "APPROVED" || lines[2]["session_id"] != "sess-1" {
		t.Errorf("the verdict: %v", lines[2])
	}
	// The secret, and the keys of the decision (which would be there if the body were logged). Digits are not checked: the
	// log's own timestamps are full of them.
	for _, leak := range []string{diditTestSecret, "decision", "score"} {
		if strings.Contains(buf.String(), leak) {
			t.Errorf("the log holds %q:\n%s", leak, buf.String())
		}
	}
}

type fakeSessions struct{ list []store.Session }

func (f fakeSessions) Sessions(_ context.Context, onboardingID string) ([]store.Session, error) {
	var out []store.Session
	for _, s := range f.list {
		if s.OnboardingID == onboardingID {
			out = append(out, s)
		}
	}
	return out, nil
}

// GetOnboarding returns the hosted verification links, so whoever gives a person their link can read it.
func TestGetOnboardingReturnsTheVerificationLinks(t *testing.T) {
	tc := newFakeTemporal()
	s := New(tc, mustFake(t), "onboarding")
	start, err := s.StartBusinessOnboarding(context.Background(), request("k-links", nil))
	if err != nil {
		t.Fatal(err)
	}
	id := start.Msg.OnboardingId
	s.WithSessions(fakeSessions{[]store.Session{
		{OnboardingID: id, SubjectPartyID: "company-1", Kind: "business", SessionID: "s-co", URL: "https://verify.example/co"},
		{OnboardingID: id, SubjectPartyID: "person-1", Kind: "person", SessionID: "s-p", URL: "https://verify.example/p"},
		{OnboardingID: "another-onboarding", SubjectPartyID: "x", Kind: "person", SessionID: "s-x", URL: "https://verify.example/x"},
	}})
	res, err := s.GetOnboarding(context.Background(), connect.NewRequest(&onboardingv1.GetOnboardingRequest{OnboardingId: id}))
	if err != nil {
		t.Fatal(err)
	}
	links := res.Msg.Status.VerificationLinks
	if len(links) != 2 || links[0].Kind != "business" || links[0].Url != "https://verify.example/co" || links[1].SubjectPartyId != "person-1" {
		t.Fatalf("links = %+v; only this onboarding's, with their URLs", links)
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
