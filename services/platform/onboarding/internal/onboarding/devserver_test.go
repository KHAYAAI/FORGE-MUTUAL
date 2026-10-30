//go:build temporaldev

package onboarding_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"testing"
	"time"

	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"github.com/khayaai/forge-mutual/services/platform/onboarding/internal/onboarding"
	"github.com/khayaai/forge-mutual/services/platform/onboarding/internal/onboarding/fakes"
)

// These tests run the workflow on a real Temporal server. Start a dev server and set TEMPORAL_HOST:
//
//	temporal server start-dev --headless --port 7233
//	TEMPORAL_HOST=127.0.0.1:7233 make test-temporal
//
// Each test uses its own task queue and workflow IDs, so the tests do not interfere with one another.

func randHex(t *testing.T) string {
	t.Helper()
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}

func devClient(t *testing.T) client.Client {
	t.Helper()
	hostPort := os.Getenv("TEMPORAL_HOST")
	if hostPort == "" {
		t.Skip("TEMPORAL_HOST not set")
	}
	c, err := client.Dial(client.Options{HostPort: hostPort, Namespace: "default"})
	if err != nil {
		t.Fatalf("dial %s: %v", hostPort, err)
	}
	t.Cleanup(c.Close)
	return c
}

// startWorker runs the workflow and the given activities on a fresh task queue.
func startWorker(t *testing.T, c client.Client, acts interface{}) string {
	t.Helper()
	queue := "onboarding-dev-" + randHex(t)
	w := worker.New(c, queue, worker.Options{})
	w.RegisterWorkflow(onboarding.OnboardBusiness)
	w.RegisterActivity(acts)
	if err := w.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(w.Stop)
	return queue
}

func input(id string) onboarding.Input {
	return onboarding.Input{
		OnboardingID: id, RequestHash: "dev", LegalName: "Forge Pilot (Pty) Ltd", RegistrationNumber: "2026/42",
		Country: "ZA", SectorCode: "software_saas", Actor: "founder",
		Directors: []onboarding.DirectorInput{
			{LegalName: "Thandi Dube", NationalID: "8001010012345", OwnershipBps: 6000, IsUBO: true},
			{LegalName: "Sipho Ndlovu", NationalID: "8002020012345", OwnershipBps: 4000},
		},
	}
}

// start begins a workflow with the same reuse policy the service uses.
func start(ctx context.Context, c client.Client, queue, id string, in onboarding.Input) (client.WorkflowRun, error) {
	return c.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID: id, TaskQueue: queue, WorkflowIDReusePolicy: enumspb.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE,
		WorkflowExecutionErrorWhenAlreadyStarted: true,
	}, onboarding.OnboardBusiness, in)
}

// waitForStep polls the status query until the workflow reaches the named step.
func waitForStep(t *testing.T, c client.Client, id, step string) onboarding.Status {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		val, err := c.QueryWorkflow(context.Background(), id, "", onboarding.QueryStatus)
		if err == nil {
			var st onboarding.Status
			if err := val.Get(&st); err != nil {
				t.Fatal(err)
			}
			for _, s := range st.Steps {
				if s == step {
					return st
				}
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("workflow %s never reached step %s", id, step)
	return onboarding.Status{}
}

// approve sends the verdict for every party, as the webhook receiver does.
func approve(t *testing.T, c client.Client, id string, st onboarding.Status) {
	t.Helper()
	for _, subject := range st.AllParties() {
		if err := c.SignalWorkflow(context.Background(), id, "", onboarding.SignalKYCResult,
			onboarding.KYCSignal{SubjectPartyID: subject, Verdict: "APPROVED"}); err != nil {
			t.Fatalf("signal %s: %v", subject, err)
		}
	}
}

func result(t *testing.T, run client.WorkflowRun) (onboarding.Status, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	var st onboarding.Status
	err := run.Get(ctx, &st)
	return st, err
}

func TestDevApprovedRunCompletes(t *testing.T) {
	c := devClient(t)
	acts := fakes.New(true)
	queue := startWorker(t, c, acts)
	id := "dev-approved-" + randHex(t)
	run, err := start(context.Background(), c, queue, id, input(id))
	if err != nil {
		t.Fatal(err)
	}
	st := waitForStep(t, c, id, "verifications_started")
	approve(t, c, id, st)
	final, err := result(t, run)
	if err != nil {
		t.Fatalf("workflow: %v", err)
	}
	if final.State != onboarding.StateCompleted || len(final.AccountIDs) != 1 {
		t.Fatalf("final = %+v", final)
	}
	if acts.Rec.Calls("CreateAccounts") != 1 || acts.Rec.Calls("ActivateParties") != 1 || acts.Rec.Calls("CloseParties") != 0 {
		t.Fatalf("calls: accounts=%d activate=%d close=%d", acts.Rec.Calls("CreateAccounts"), acts.Rec.Calls("ActivateParties"), acts.Rec.Calls("CloseParties"))
	}
}

func TestDevKYCTimeoutFiresOnTheRealTimer(t *testing.T) {
	c := devClient(t)
	acts := fakes.New(true)
	queue := startWorker(t, c, acts)
	id := "dev-timeout-" + randHex(t)
	in := input(id)
	in.KYCTimeoutSeconds = 2
	began := time.Now()
	run, err := start(context.Background(), c, queue, id, in)
	if err != nil {
		t.Fatal(err)
	}
	final, err := result(t, run)
	if err != nil {
		t.Fatalf("workflow: %v", err)
	}
	if elapsed := time.Since(began); elapsed < 2*time.Second {
		t.Fatalf("timeout fired after %s; a 2 s timer cannot fire earlier", elapsed)
	}
	if final.State != onboarding.StateDeclined || final.DeclineReason != "kyc_timeout" {
		t.Fatalf("final = %+v", final)
	}
	if acts.Rec.Calls("CloseParties") != 1 || acts.Rec.Calls("CreateAccounts") != 0 {
		t.Fatal("a timed-out application closes its parties and creates no account")
	}
}

func TestDevDuplicateStartRefusedWhileRunning(t *testing.T) {
	c := devClient(t)
	queue := startWorker(t, c, fakes.New(true))
	id := "dev-dup-" + randHex(t)
	run, err := start(context.Background(), c, queue, id, input(id))
	if err != nil {
		t.Fatal(err)
	}
	waitForStep(t, c, id, "verifications_started")
	_, err = start(context.Background(), c, queue, id, input(id))
	var already *serviceerror.WorkflowExecutionAlreadyStarted
	if !errors.As(err, &already) {
		t.Fatalf("second start while running: want WorkflowExecutionAlreadyStarted, got %v", err)
	}
	// Let the first run finish so the worker can stop cleanly.
	approve(t, c, id, waitForStep(t, c, id, "verifications_started"))
	if _, err := result(t, run); err != nil {
		t.Fatal(err)
	}
}

func TestDevFinishedWorkflowCannotBeRestarted(t *testing.T) {
	c := devClient(t)
	queue := startWorker(t, c, fakes.New(true))
	id := "dev-restart-" + randHex(t)
	in := input(id)
	in.KYCTimeoutSeconds = 1
	run, err := start(context.Background(), c, queue, id, in)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := result(t, run); err != nil {
		t.Fatal(err)
	}
	_, err = start(context.Background(), c, queue, id, input(id))
	var already *serviceerror.WorkflowExecutionAlreadyStarted
	if !errors.As(err, &already) {
		t.Fatalf("restart after completion: want refusal under REJECT_DUPLICATE, got %v", err)
	}
}

func TestDevRetryableActivityFailureIsRetried(t *testing.T) {
	c := devClient(t)
	acts := fakes.New(true)
	acts.AccountFailures = 2 // fails twice; the retry policy allows five attempts
	queue := startWorker(t, c, acts)
	id := "dev-retry-" + randHex(t)
	run, err := start(context.Background(), c, queue, id, input(id))
	if err != nil {
		t.Fatal(err)
	}
	approve(t, c, id, waitForStep(t, c, id, "verifications_started"))
	final, err := result(t, run)
	if err != nil {
		t.Fatalf("workflow should recover after retries: %v", err)
	}
	if final.State != onboarding.StateCompleted {
		t.Fatalf("final = %+v", final)
	}
	if n := acts.Rec.Calls("CreateAccounts"); n != 3 {
		t.Fatalf("CreateAccounts attempts = %d, want 3", n)
	}
}

func TestDevNonRetryableActivityFailureStopsAtOnce(t *testing.T) {
	c := devClient(t)
	acts := fakes.New(true)
	acts.AccountFatal = true
	queue := startWorker(t, c, acts)
	id := "dev-fatal-" + randHex(t)
	run, err := start(context.Background(), c, queue, id, input(id))
	if err != nil {
		t.Fatal(err)
	}
	approve(t, c, id, waitForStep(t, c, id, "verifications_started"))
	if _, err := result(t, run); err == nil {
		t.Fatal("a non-retryable ledger refusal must fail the run")
	}
	if n := acts.Rec.Calls("CreateAccounts"); n != 1 {
		t.Fatalf("CreateAccounts attempts = %d, want 1 (non-retryable)", n)
	}
}
