package risk

import (
	"testing"
	"time"
)

func snap(deposits, largest, pool, suspense int64) Snapshot {
	return Snapshot{Asset: "ZAR", At: time.Now(), Deposits: deposits, LargestDeposit: largest, LargestAccount: "acc-1", Pool: pool, Suspense: suspense}
}

func kinds(as []Alert) map[string]bool {
	m := map[string]bool{}
	for _, a := range as {
		m[a.Kind] = true
	}
	return m
}

// The limit is inclusive: exactly 25 percent is a concentration.
func TestConcentrationBoundaryIsInclusive(t *testing.T) {
	th := DefaultThresholds()
	at := Evaluate(snap(10000, 2500, 10000, 0), nil, th)
	if !kinds(at)[KindConcentration] {
		t.Fatalf("exactly at the limit must alert: %+v", at)
	}
	below := Evaluate(snap(10000, 2499, 10000, 0), nil, th)
	if kinds(below)[KindConcentration] {
		t.Fatalf("just below the limit must not alert: %+v", below)
	}
}

func TestNetOutflowNeedsAPriorSnapshot(t *testing.T) {
	th := DefaultThresholds()
	cur := snap(7000, 0, 7000, 0)
	if kinds(Evaluate(cur, nil, th))[KindNetOutflow] {
		t.Fatal("no prior snapshot must not raise an outflow alert")
	}
	prev := snap(10000, 0, 10000, 0)
	if !kinds(Evaluate(cur, &prev, th))[KindNetOutflow] {
		t.Fatal("a 30 percent fall must raise an outflow alert")
	}
	mild := snap(9000, 0, 9000, 0)
	if kinds(Evaluate(mild, &prev, th))[KindNetOutflow] {
		t.Fatal("a 10 percent fall must not raise an outflow alert")
	}
}

// Deposits rising is not an outflow.
func TestGrowthIsNotOutflow(t *testing.T) {
	prev := snap(10000, 0, 20000, 0)
	cur := snap(15000, 0, 20000, 0)
	if kinds(Evaluate(cur, &prev, DefaultThresholds()))[KindNetOutflow] {
		t.Fatal("growing deposits must not raise an outflow alert")
	}
}

// Deposits above the pool mean the books do not reconcile with the bank. That is critical.
func TestLiquidityBreachIsCritical(t *testing.T) {
	as := Evaluate(snap(10000, 0, 9999, 0), nil, DefaultThresholds())
	for _, a := range as {
		if a.Kind == KindLiquidity && a.Severity == SeverityCritical {
			return
		}
	}
	t.Fatalf("deposits above the pool must be critical: %+v", as)
}

func TestSuspenseAboveLimitAlerts(t *testing.T) {
	th := DefaultThresholds()
	if kinds(Evaluate(snap(0, 0, 0, th.SuspenseMinor), nil, th))[KindSuspenseAge] {
		t.Fatal("suspense at the limit must not alert")
	}
	if !kinds(Evaluate(snap(0, 0, 0, th.SuspenseMinor+1), nil, th))[KindSuspenseAge] {
		t.Fatal("suspense above the limit must alert")
	}
}

// A quiet book raises nothing.
func TestQuietBookRaisesNothing(t *testing.T) {
	prev := snap(10000, 1000, 10000, 0)
	cur := snap(10100, 1000, 10100, 0)
	if as := Evaluate(cur, &prev, DefaultThresholds()); len(as) != 0 {
		t.Fatalf("quiet book raised %+v", as)
	}
}

func TestBpsNeverOverflows(t *testing.T) {
	big := int64(1) << 60
	if got := Bps(big, big*2); got != 5000 {
		t.Fatalf("Bps = %d, want 5000", got)
	}
	if got := Bps(5, 0); got != 0 {
		t.Fatalf("zero whole = %d", got)
	}
}
