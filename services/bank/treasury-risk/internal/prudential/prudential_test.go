package prudential

import "testing"

// A small, well-capitalised bank: the arithmetic of each ratio, checked by hand.
func TestTheRatiosAreComputedAsStated(t *testing.T) {
	pos := Position{
		Capital: 25_000_000_00, Cash: 80_000_000_00, BusinessLoans: 40_000_000_00, UndrawnOverdft: 10_000_000_00,
		CurrentBusiness: 60_000_000_00, CurrentRetail: 20_000_000_00, Savings: 20_000_000_00, NoticeLonger: 10_000_000_00,
		NoticeDue30: 1_000_000_00, InstalmentsDue30: 4_000_000_00, NetOpenFX: 1_000_000_00, GrossIncome: 8_000_000_00,
		LargestDepositor: 15_000_000_00,
	}
	r := Compute(pos, DefaultParams())
	// Credit RWA: cash 20% (16m) + loans 100% (40m) + undrawn 10% CCF at 100% (1m) = 57m.
	if r.CreditRWA != 57_000_000_00 {
		t.Fatalf("credit RWA = %d", r.CreditRWA)
	}
	// Operational: 12.5 x 15% x 8m = 15m. Market: the open FX position, 1m. Total 73m. Capital 25m / 73m = 34.24%.
	if r.OperationalRWA != 15_000_000_00 || r.TotalRWA != 73_000_000_00 || r.TotalCapitalBps != 3424 {
		t.Fatalf("op %d total %d ratio %d", r.OperationalRWA, r.TotalRWA, r.TotalCapitalBps)
	}
	// Outflows: business 40% (24m) + retail 10% (2m) + savings 10% (2m) + notice due (1m) + undrawn 10% (1m) = 30m.
	// Inflows: half the 4m due = 2m. Net 28m. LCR = 80m / 28m = 285.71%.
	if r.Outflows30 != 30_000_000_00 || r.NetOutflows30 != 28_000_000_00 || r.LCRBps != 28571 {
		t.Fatalf("outflows %d net %d lcr %d", r.Outflows30, r.NetOutflows30, r.LCRBps)
	}
	if len(r.Breaches) != 0 {
		t.Fatalf("breaches %v", r.Breaches)
	}
	for _, s := range r.Stress {
		if s.Name == "RUN_MODERATE" && (!s.Survives || s.LiquidityLeft != 50_000_000_00) {
			t.Fatalf("moderate run: %+v", s)
		}
	}
}

// A thinly capitalised, illiquid bank breaches every minimum, and fails the severe scenarios.
func TestBreachesAndFailingScenariosAreReported(t *testing.T) {
	pos := Position{Capital: 1_000_000_00, Cash: 5_000_000_00, BusinessLoans: 50_000_000_00, CurrentBusiness: 50_000_000_00, LargestDepositor: 20_000_000_00}
	r := Compute(pos, DefaultParams())
	want := map[string]bool{"TOTAL_CAPITAL_RATIO": true, "LEVERAGE_RATIO": true, "LIQUIDITY_COVERAGE": true}
	if len(r.Breaches) != 3 {
		t.Fatalf("breaches = %v", r.Breaches)
	}
	for _, b := range r.Breaches {
		if !want[b] {
			t.Fatalf("unexpected breach %s", b)
		}
	}
	for _, s := range r.Stress {
		if s.Survives {
			t.Fatalf("%s survived on R10 000 of capital", s.Name)
		}
	}
}

// With nothing at risk, ratios are unbounded and nothing breaches: an empty bank is not in breach.
func TestAnEmptyBankIsNotInBreach(t *testing.T) {
	r := Compute(Position{}, DefaultParams())
	if len(r.Breaches) != 0 {
		t.Fatalf("breaches = %v", r.Breaches)
	}
}

// The legal form's minimum capital is a breach of its own: a mutual bank needs R10 million, a bank R250 million.
func TestCapitalBelowTheLegalMinimumIsABreach(t *testing.T) {
	pos := Position{Capital: 50_000_000 * 100}
	p := DefaultParams()
	p.MinInitialCapital = 10_000_000 * 100 // mutual bank
	for _, b := range Compute(pos, p).Breaches {
		if b == "INITIAL_CAPITAL" {
			t.Fatal("R50 million is above a mutual bank's minimum")
		}
	}
	p.MinInitialCapital = 250_000_000 * 100 // bank
	found := false
	for _, b := range Compute(pos, p).Breaches {
		found = found || b == "INITIAL_CAPITAL"
	}
	if !found {
		t.Fatal("R50 million is below a bank's minimum")
	}
}
