package router

import (
	"errors"
	"testing"
)

func win(outcomes ...bool) Window { return Window{Outcomes: outcomes} }

// Three failures in a row mean down, whatever the earlier history.
func TestThreeConsecutiveFailuresAreDown(t *testing.T) {
	if s := win(true, true, true, true, true, false, false, false).State(); s != Down {
		t.Fatalf("state = %s, want DOWN", s)
	}
	if s := win(false, false, true).State(); s == Down {
		t.Fatal("two failures and a success must not be down")
	}
}

// A partner with too few samples is not judged; with enough, the rate decides.
func TestRateDecidesWithEnoughSamples(t *testing.T) {
	if s := win(false, true, false, true).State(); s != Healthy {
		t.Fatalf("four samples are not enough to judge: %s", s)
	}
	degraded := win(true, true, true, true, true, true, true, true, false, true) // 90 percent
	if s := degraded.State(); s != Healthy {
		t.Fatalf("exactly 90 percent is healthy, got %s", s)
	}
	poor := win(true, true, false, false, true, false, false, true, false, false) // four in ten: 40 percent
	if s := poor.State(); s != Down {
		t.Fatalf("below 50 percent is down, got %s", s)
	}
}

// The cheapest healthy partner wins; a down partner is never chosen even if it is cheapest.
func TestCheapestHealthyWinsAndDownIsSkipped(t *testing.T) {
	cands := []Candidate{
		{Name: "cheap-but-down", Asset: "USD", CostBps: 5, Window: win(false, false, false)},
		{Name: "mid", Asset: "USD", CostBps: 20, Window: win()},
		{Name: "dear", Asset: "USD", CostBps: 40, Window: win()},
	}
	c, err := Choose("USD", cands)
	if err != nil || c.Name != "mid" {
		t.Fatalf("chose %+v, %v; want mid", c, err)
	}
}

// A healthy partner beats a cheaper degraded one.
func TestHealthyBeatsCheaperDegraded(t *testing.T) {
	cands := []Candidate{
		{Name: "cheap-degraded", Asset: "USD", CostBps: 1, Window: win(true, true, true, true, true, true, false, false, true, false)},
		{Name: "dear-healthy", Asset: "USD", CostBps: 30, Window: win()},
	}
	c, _ := Choose("USD", cands)
	if c.Name != "dear-healthy" {
		t.Fatalf("chose %s", c.Name)
	}
}

// With no partner available for the asset the operation is refused. It is never sent to another asset's partner.
func TestNoPartnerRefuses(t *testing.T) {
	cands := []Candidate{{Name: "zar-only", Asset: "ZAR", CostBps: 1, Window: win()}}
	if _, err := Choose("USD", cands); !errors.Is(err, ErrNoPartner) {
		t.Fatalf("err = %v, want ErrNoPartner", err)
	}
}
