//go:build temporaldev

package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	onboardingv1 "github.com/khayaai/forge-mutual/pkg/api/forge/onboarding/v1"
	"github.com/khayaai/forge-mutual/services/platform/onboarding/internal/kyc"
	"github.com/khayaai/forge-mutual/services/platform/onboarding/internal/onboarding"
	"github.com/khayaai/forge-mutual/services/platform/onboarding/internal/onboarding/fakes"
)

// These tests drive the onboarding API against a real Temporal server, through the real client.
// Start a dev server and run: TEMPORAL_HOST=127.0.0.1:7233 make test-temporal

const devSecret = "0123456789abcdef-dev-webhook-secret"

func devRand(t *testing.T) string {
	t.Helper()
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}

// devService returns the service wired to a real client and a worker running the workflow with fake
// activities on a fresh task queue, with the fake KYC provider.
func devService(t *testing.T) (*Service, client.Client) {
	t.Helper()
	provider, err := kyc.NewFake([]byte(devSecret))
	if err != nil {
		t.Fatal(err)
	}
	return devServiceWith(t, provider)
}

// devServiceWith is devService with the given KYC provider.
func devServiceWith(t *testing.T, provider kyc.Provider) (*Service, client.Client) {
	t.Helper()
	hostPort := os.Getenv("TEMPORAL_HOST")
	if hostPort == "" {
		t.Skip("TEMPORAL_HOST not set")
	}
	c, err := client.Dial(client.Options{HostPort: hostPort, Namespace: "default"})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(c.Close)
	queue := "onboarding-svc-dev-" + devRand(t)
	w := worker.New(c, queue, worker.Options{})
	w.RegisterWorkflow(onboarding.OnboardBusiness)
	w.RegisterActivity(fakes.New(true))
	if err := w.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(w.Stop)
	return New(c, provider, queue), c
}

func devRequest(key, legalName string) *connect.Request[onboardingv1.StartBusinessOnboardingRequest] {
	return connect.NewRequest(&onboardingv1.StartBusinessOnboardingRequest{
		IdempotencyKey: key, LegalName: legalName, RegistrationNumber: "2026/88", Country: "ZA",
		SectorCode: "software_saas", Actor: "founder",
		Directors: []*onboardingv1.Director{
			{LegalName: "Thandi Dube", NationalId: "8001010012345", OwnershipBps: 6000, IsUbo: true},
		},
	})
}

// waitForSteps polls GetOnboarding until the workflow has reached the named step.
func waitForSteps(t *testing.T, s *Service, id, step string) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		res, err := s.GetOnboarding(context.Background(), connect.NewRequest(&onboardingv1.GetOnboardingRequest{OnboardingId: id}))
		if err == nil {
			for _, got := range res.Msg.Status.Steps {
				if got == step {
					return
				}
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("onboarding %s never reached %s", id, step)
}

func TestDevStartIsIdempotentOnTheRealServer(t *testing.T) {
	s, _ := devService(t)
	key := "dev-idem-" + devRand(t)
	first, err := s.StartBusinessOnboarding(context.Background(), devRequest(key, "Forge Pilot (Pty) Ltd"))
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.StartBusinessOnboarding(context.Background(), devRequest(key, "Forge Pilot (Pty) Ltd"))
	if err != nil {
		t.Fatalf("same key and request must replay, got %v", err)
	}
	if again.Msg.OnboardingId != first.Msg.OnboardingId {
		t.Fatal("replay returned a different onboarding")
	}
}

// Before the already-started fix, the SDK returned the existing run with no error here, so a changed
// request under the same key was accepted. This test guards that path.
func TestDevSameKeyDifferentRequestIsRefusedOnTheRealServer(t *testing.T) {
	s, _ := devService(t)
	key := "dev-conflict-" + devRand(t)
	if _, err := s.StartBusinessOnboarding(context.Background(), devRequest(key, "Forge Pilot (Pty) Ltd")); err != nil {
		t.Fatal(err)
	}
	_, err := s.StartBusinessOnboarding(context.Background(), devRequest(key, "A Different Company (Pty) Ltd"))
	if connect.CodeOf(err) != connect.CodeAlreadyExists {
		t.Fatalf("changed request under the same key: want AlreadyExists, got %v", err)
	}
}

func TestDevGetOnboardingReportsRunningThenUnknown(t *testing.T) {
	s, _ := devService(t)
	started, err := s.StartBusinessOnboarding(context.Background(), devRequest("dev-get-"+devRand(t), "Forge Pilot (Pty) Ltd"))
	if err != nil {
		t.Fatal(err)
	}
	waitForSteps(t, s, started.Msg.OnboardingId, "verifications_started")
	res, err := s.GetOnboarding(context.Background(), connect.NewRequest(&onboardingv1.GetOnboardingRequest{OnboardingId: started.Msg.OnboardingId}))
	if err != nil || res.Msg.Status.State != onboardingv1.OnboardingState_ONBOARDING_STATE_RUNNING {
		t.Fatalf("running onboarding: %v %v", res, err)
	}
	_, err = s.GetOnboarding(context.Background(), connect.NewRequest(&onboardingv1.GetOnboardingRequest{OnboardingId: "11111111-1111-4111-8111-111111111111"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("unknown onboarding: want NotFound, got %v", err)
	}
}

// signedWebhook posts a verdict the way the provider does: the body and its HMAC signature.
func signedWebhook(t *testing.T, h http.Handler, onboardingID, subject, verdict string) int {
	t.Helper()
	body := `{"session_id":"s","onboarding_id":"` + onboardingID + `","subject_party_id":"` + subject + `","verdict":"` + verdict + `"}`
	r := httptest.NewRequest(http.MethodPost, "/webhooks/kyc", strings.NewReader(body))
	r.Header.Set("X-Forge-Signature", kyc.Sign([]byte(devSecret), []byte(body)))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w.Code
}

func TestDevWebhookVerdictsCompleteTheOnboarding(t *testing.T) {
	s, c := devService(t)
	h := s.WebhookHandler()
	started, err := s.StartBusinessOnboarding(context.Background(), devRequest("dev-hook-"+devRand(t), "Forge Pilot (Pty) Ltd"))
	if err != nil {
		t.Fatal(err)
	}
	id := started.Msg.OnboardingId
	waitForSteps(t, s, id, "verifications_started")

	// A forged signature changes nothing.
	body := `{"session_id":"s","onboarding_id":"` + id + `","subject_party_id":"company-1","verdict":"APPROVED"}`
	r := httptest.NewRequest(http.MethodPost, "/webhooks/kyc", strings.NewReader(body))
	r.Header.Set("X-Forge-Signature", "deadbeef")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("forged verdict: %d", w.Code)
	}

	// A signed verdict for an onboarding that does not exist (or has ended) is acknowledged with 200, not 404: it is
	// authentic and moot, and a provider told 404 would retry it for nothing. This is the real server's NotFound.
	if code := signedWebhook(t, h, "11111111-1111-4111-8111-111111111111", "company-1", "APPROVED"); code != http.StatusOK {
		t.Fatalf("unknown onboarding webhook: %d, want 200", code)
	}

	for _, subject := range []string{"company-1", "person-1", "person-2"} {
		if code := signedWebhook(t, h, id, subject, "APPROVED"); code != http.StatusAccepted {
			t.Fatalf("approval for %s: %d", subject, code)
		}
	}
	run := c.GetWorkflow(context.Background(), id, "")
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	var final onboarding.Status
	if err := run.Get(ctx, &final); err != nil {
		t.Fatalf("workflow: %v", err)
	}
	if final.State != onboarding.StateCompleted {
		t.Fatalf("final = %+v", final)
	}
}

// The same journey with Didit as the provider: deliveries signed as Didit signs them, handled by the real handler and
// the real Temporal server. Everything that must not reach the workflow is refused on the way, and what is moot is
// acknowledged so that Didit does not retry it.
func TestDevDiditVerdictsCompleteTheOnboarding(t *testing.T) {
	const secret = "whsec_dev_server_test_secret_0123456789"
	provider, err := kyc.NewDidit(kyc.DiditConfig{
		APIKey: "unused", KYCWorkflowID: "11111111-1111-4111-8111-111111111111", KYBWorkflowID: "22222222-2222-4222-8222-222222222222",
		WebhookSecret: secret, Environment: "live",
	})
	if err != nil {
		t.Fatal(err)
	}
	s, c := devServiceWith(t, provider)
	h := s.WebhookHandler()
	started, err := s.StartBusinessOnboarding(context.Background(), devRequest("dev-didit-"+devRand(t), "Forge Pilot (Pty) Ltd"))
	if err != nil {
		t.Fatal(err)
	}
	id := started.Msg.OnboardingId
	waitForSteps(t, s, id, "verifications_started")

	send := func(onboardingID, subject, status, environment string, mod func(http.Header)) int {
		t.Helper()
		body, err := json.Marshal(map[string]any{
			"event_id": "dev-" + devRand(t), "webhook_type": "status.updated", "environment": environment,
			"session_id": "sess-" + subject, "status": status, "vendor_data": subject,
			"metadata": map[string]any{"onboarding_id": onboardingID, "subject_party_id": subject},
		})
		if err != nil {
			t.Fatal(err)
		}
		sig, err := kyc.SignDidit(secret, body)
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest(http.MethodPost, "/webhooks/kyc", bytes.NewReader(body))
		r.Header.Set("X-Signature-V2", sig)
		r.Header.Set("X-Timestamp", strconv.FormatInt(time.Now().Unix(), 10))
		if mod != nil {
			mod(r.Header)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}

	// Sent in this order. Each is refused or acknowledged, and none may reach the workflow.
	for _, tc := range []struct {
		name      string
		want, got int
	}{
		{"forged", http.StatusUnauthorized, send(id, "company-1", "Approved", "live", func(h http.Header) { h.Set("X-Signature-V2", strings.Repeat("0", 64)) })},
		{"stale", http.StatusUnauthorized, send(id, "company-1", "Approved", "live", func(h http.Header) {
			h.Set("X-Timestamp", strconv.FormatInt(time.Now().Add(-time.Hour).Unix(), 10))
		})},
		{"a sandbox verdict at a live service", http.StatusBadRequest, send(id, "company-1", "Approved", "sandbox", nil)},
		{"not a verdict", http.StatusOK, send(id, "company-1", "In Progress", "live", nil)},
		{"an onboarding that does not exist", http.StatusOK, send("11111111-1111-4111-8111-111111111111", "company-1", "Approved", "live", nil)},
	} {
		if tc.got != tc.want {
			t.Fatalf("%s: %d, want %d", tc.name, tc.got, tc.want)
		}
	}
	// None of that reached the workflow: it is still waiting on every subject.
	res, err := s.GetOnboarding(context.Background(), connect.NewRequest(&onboardingv1.GetOnboardingRequest{OnboardingId: id}))
	if err != nil || res.Msg.Status.State != onboardingv1.OnboardingState_ONBOARDING_STATE_RUNNING {
		t.Fatalf("after refused deliveries: %v %v", res, err)
	}
	for _, step := range res.Msg.Status.Steps {
		if step == "kyc_approved" {
			t.Fatalf("a delivery that must not count approved a subject: %v", res.Msg.Status.Steps)
		}
	}

	for _, subject := range []string{"company-1", "person-1", "person-2"} {
		if code := send(id, subject, "Approved", "live", nil); code != http.StatusAccepted {
			t.Fatalf("approval for %s: %d", subject, code)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	var final onboarding.Status
	if err := c.GetWorkflow(context.Background(), id, "").Get(ctx, &final); err != nil {
		t.Fatalf("workflow: %v", err)
	}
	if final.State != onboarding.StateCompleted {
		t.Fatalf("final = %+v", final)
	}
	// Didit retries what it is not sure arrived. A verdict that comes after the onboarding has ended is acknowledged.
	if code := send(id, "company-1", "Approved", "live", nil); code != http.StatusOK {
		t.Fatalf("a verdict after the onboarding ended: %d, want 200", code)
	}
}
