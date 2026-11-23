package returns

import (
	"errors"
	"testing"
	"time"
)

var t0 = time.Date(2026, 11, 30, 12, 0, 0, 0, time.UTC)

func spec(verified bool) Spec {
	rd := func(n string) func(map[string]Cents) (Cents, error) {
		return func(in map[string]Cents) (Cents, error) { return in[n], nil }
	}
	return Spec{
		Form: "TEST 1", Frequency: "monthly",
		Lines: []Line{
			{Ref: "L1", Inputs: []string{"cash"}, Compute: rd("cash"), Verified: verified, CheckedBy: "x"},
			{Ref: "L2", Inputs: []string{"loans"}, Compute: rd("loans"), Verified: verified, CheckedBy: "x"},
			{Ref: "L3", Inputs: []string{"total"}, Compute: rd("total"), Verified: verified, CheckedBy: "x"},
		},
		Totals:    []Total{{Line: "L3", Sum: []string{"L1", "L2"}}},
		Reconcile: map[string]func(map[string]Cents) (Cents, error){"L1": rd("ledger_cash")},
	}
}

func TestComputeClean(t *testing.T) {
	r, err := spec(true).Compute("2026-10", map[string]Cents{"cash": 100, "loans": 50, "total": 150, "ledger_cash": 100}, t0)
	if err != nil || len(r.Breaks) != 0 || len(r.Unverified) != 0 {
		t.Fatalf("%v %+v", err, r)
	}
}

func TestMissingInputRefused(t *testing.T) {
	_, err := spec(true).Compute("p", map[string]Cents{"cash": 1}, t0)
	if !errors.Is(err, ErrIncomplete) {
		t.Fatalf("want ErrIncomplete, got %v", err)
	}
}

func TestArithmeticAndReconciliationBreaks(t *testing.T) {
	r, err := spec(true).Compute("p", map[string]Cents{"cash": 100, "loans": 50, "total": 149, "ledger_cash": 90}, t0)
	if err != nil || len(r.Breaks) != 2 {
		t.Fatalf("want 2 breaks, got %v %+v", err, r.Breaks)
	}
	if _, err := Prepare(r, "alice", "digest"); !errors.Is(err, ErrBreak) {
		t.Fatalf("a return with breaks must not be prepared: %v", err)
	}
}

func TestUnverifiedLinesBlockSubmission(t *testing.T) {
	r, _ := spec(false).Compute("p", map[string]Cents{"cash": 1, "loans": 1, "total": 2, "ledger_cash": 1}, t0)
	if len(r.Unverified) != 3 {
		t.Fatalf("unverified: %v", r.Unverified)
	}
	c, err := Prepare(r, "alice", "digest")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Approve("bob", t0); err != nil {
		t.Fatal(err)
	}
	if err := c.Submittable(); !errors.Is(err, ErrUnverified) {
		t.Fatalf("want ErrUnverified, got %v", err)
	}
}

func TestApprovalNeedsDistinctSigner(t *testing.T) {
	r, _ := spec(true).Compute("p", map[string]Cents{"cash": 1, "loans": 1, "total": 2, "ledger_cash": 1}, t0)
	c, _ := Prepare(r, "Alice", "digest")
	if err := c.Approve(" alice ", t0); !errors.Is(err, ErrSameSigner) {
		t.Fatalf("same person approved: %v", err)
	}
	if err := c.Submittable(); !errors.Is(err, ErrNotApproved) {
		t.Fatalf("unapproved: %v", err)
	}
	if err := c.Approve("bob", t0); err != nil {
		t.Fatal(err)
	}
	if err := c.Submittable(); err != nil {
		t.Fatalf("verified, approved, clean return should be submittable: %v", err)
	}
}

func TestPrepareNeedsEvidence(t *testing.T) {
	r, _ := spec(true).Compute("p", map[string]Cents{"cash": 1, "loans": 1, "total": 2, "ledger_cash": 1}, t0)
	if _, err := Prepare(r, "alice", ""); !errors.Is(err, ErrIncomplete) {
		t.Fatalf("no digest: %v", err)
	}
}
