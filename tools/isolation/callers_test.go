package main

import (
	"strings"
	"testing"
)

const ledgerCaller = `package recon

import (
	"context"

	ledgerv1 "github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1"
)

type client interface {
	GetBalances(context.Context, *ledgerv1.GetBalancesRequest) error
	CreateTransfers(context.Context, *ledgerv1.CreateTransfersRequest) error
}

func run(ctx context.Context, c client) error { return c.GetBalances(ctx, nil) }
`

func plantCallers(t *testing.T, root, target, body string) {
	t.Helper()
	write(t, root, "deploy/compose/config/callers/"+target+".json", body)
}

// The case this check exists for: reconciliation reads the ledger, and was allowed to write to it.
func TestAGrantTheCallerNeverUsesIsAViolation(t *testing.T) {
	root := t.TempDir()
	write(t, root, "services/bank/reconciliation/internal/recon/recon.go", ledgerCaller)
	plantCallers(t, root, "ledger", `{"svc-reconciliation": ["GetBalances", "CreateTransfers"]}`)
	v, err := checkCallers(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(v) != 1 || !strings.Contains(v[0], "svc-reconciliation is granted CreateTransfers") {
		t.Fatalf("want one violation for the unused CreateTransfers grant, got %v", v)
	}
}

func TestGrantsTheCallerUsesPass(t *testing.T) {
	root := t.TempDir()
	write(t, root, "services/bank/reconciliation/internal/recon/recon.go", ledgerCaller)
	plantCallers(t, root, "ledger", `{"svc-reconciliation": ["GetBalances"]}`)
	plantCallers(t, root, "audit", `{}`) // deny everyone: nothing to justify
	if v, err := checkCallers(root); err != nil || len(v) != 0 {
		t.Fatalf("want no violations, got %v %v", v, err)
	}
}

// A call in a test does not make a service need the permission in production.
func TestACallInATestFileDoesNotJustifyAGrant(t *testing.T) {
	root := t.TempDir()
	write(t, root, "services/bank/reconciliation/internal/recon/recon.go", ledgerCaller)
	write(t, root, "services/bank/reconciliation/internal/recon/recon_test.go", strings.Replace(ledgerCaller,
		"return c.GetBalances(ctx, nil)", "return c.CreateTransfers(ctx, nil)", 1))
	plantCallers(t, root, "ledger", `{"svc-reconciliation": ["GetBalances", "CreateTransfers"]}`)
	v, err := checkCallers(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(v) != 1 || !strings.Contains(v[0], "CreateTransfers") {
		t.Fatalf("want the CreateTransfers grant refused, got %v", v)
	}
}

// A method of the same name called on some other service does not count: the file must import the target's API.
func TestACallToAnotherServiceDoesNotJustifyAGrant(t *testing.T) {
	root := t.TempDir()
	write(t, root, "services/bank/reconciliation/internal/recon/recon.go", strings.Replace(ledgerCaller,
		"pkg/api/forge/ledger/v1", "pkg/api/forge/payments/v1", 1))
	plantCallers(t, root, "ledger", `{"svc-reconciliation": ["GetBalances"]}`)
	v, err := checkCallers(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(v) != 1 || !strings.Contains(v[0], "never calls on ledger") {
		t.Fatalf("want the grant refused, got %v", v)
	}
}

func TestAnUnknownCallerAndAnEmptyGrantAreViolations(t *testing.T) {
	root := t.TempDir()
	plantCallers(t, root, "ledger", `{"svc-somebody-new": ["GetBalances"], "svc-reconciliation": []}`)
	v, err := checkCallers(root)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(v, "\n")
	if len(v) != 2 || !strings.Contains(joined, "unknown caller svc-somebody-new") || !strings.Contains(joined, "listed with no methods") {
		t.Fatalf("want an unknown caller and an empty grant, got %v", v)
	}
}

// The journey tests are a caller too. Their grants must be calls they make.
func TestAHarnessGrantMustBeACallTheHarnessMakes(t *testing.T) {
	root := t.TempDir()
	write(t, root, "deploy/compose/journeys/seed.cjs", `await connect("GetOnboarding", { onboardingId });`)
	plantCallers(t, root, "onboarding", `{"svc-stack-journeys": ["GetOnboarding", "DecideKYCReview"]}`)
	v, err := checkCallers(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(v) != 1 || !strings.Contains(v[0], "DecideKYCReview") {
		t.Fatalf("want the unused harness grant refused, got %v", v)
	}
}
