package monitor

import (
	"strings"
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)

func ev(id, party, kind string, at time.Duration, amt int64, ben string) Event {
	return Event{ID: id, PartyID: party, Kind: kind, At: t0.Add(at), AmountMinor: amt, Beneficiary: ben}
}

func rules(fs []Finding) map[string]int {
	m := map[string]int{}
	for _, f := range fs {
		m[f.Rule]++
	}
	return m
}

func TestQuietPartyRaisesNothing(t *testing.T) {
	evs := []Event{ev("1", "p", KindPaymentOut, 0, 10_000, "supplier")}
	if fs := Evaluate(evs, DefaultConfig()); len(fs) != 0 {
		t.Fatalf("quiet party raised %+v", fs)
	}
}

// Exactly at the velocity limit is a finding; one minor unit below is not.
func TestVelocityBoundary(t *testing.T) {
	cfg := DefaultConfig()
	at := []Event{ev("1", "p", KindPaymentOut, 0, cfg.VelocityMinor, "a")}
	if rules(Evaluate(at, cfg))[RuleVelocity] != 1 {
		t.Fatal("exactly at the limit must fire")
	}
	below := []Event{ev("1", "p", KindPaymentOut, 0, cfg.VelocityMinor-1, "a")}
	if rules(Evaluate(below, cfg))[RuleVelocity] != 0 {
		t.Fatal("one unit below must not fire")
	}
}

// Velocity counts only payments inside the window.
func TestVelocityIgnoresPaymentsOutsideTheWindow(t *testing.T) {
	cfg := DefaultConfig()
	half := cfg.VelocityMinor / 2
	evs := []Event{
		ev("1", "p", KindPaymentOut, 0, half, "a"),
		ev("2", "p", KindPaymentOut, 25*time.Hour, half+1, "a"),
	}
	if rules(Evaluate(evs, cfg))[RuleVelocity] != 0 {
		t.Fatal("payments more than a window apart must not add up")
	}
}

// Three payments just under the band within a week is structuring.
func TestStructuringNeedsTheBandAndTheCount(t *testing.T) {
	cfg := DefaultConfig()
	band := cfg.StructuringLowMinor
	evs := []Event{
		ev("1", "p", KindPaymentOut, 0, band, "a"),
		ev("2", "p", KindPaymentOut, 24*time.Hour, band+1, "a"),
		ev("3", "p", KindPaymentOut, 48*time.Hour, band+2, "a"),
	}
	if rules(Evaluate(evs, cfg))[RuleStructuring] != 1 {
		t.Fatal("three payments in the band must fire")
	}
	outside := []Event{
		ev("1", "p", KindPaymentOut, 0, band-1, "a"),
		ev("2", "p", KindPaymentOut, 24*time.Hour, band-1, "a"),
		ev("3", "p", KindPaymentOut, 48*time.Hour, cfg.StructuringHighMinor, "a"),
	}
	if rules(Evaluate(outside, cfg))[RuleStructuring] != 0 {
		t.Fatal("amounts outside the band must not fire")
	}
}

func TestNewBeneficiaryFiresOnceForALargeFirstPayment(t *testing.T) {
	cfg := DefaultConfig()
	evs := []Event{
		ev("1", "p", KindPaymentOut, 0, cfg.LargeNewBeneficiaryMinor, "new-co"),
		ev("2", "p", KindPaymentOut, time.Hour, cfg.LargeNewBeneficiaryMinor, "new-co"),
		ev("3", "p", KindPaymentOut, 2*time.Hour, 100, "small-new"),
	}
	if n := rules(Evaluate(evs, cfg))[RuleNewBeneficiary]; n != 1 {
		t.Fatalf("new beneficiary fired %d times, want 1 (first large payment only)", n)
	}
}

// Money in and mostly out again within a day is the mule pattern.
func TestRapidInOutFiresAboveTheRatio(t *testing.T) {
	cfg := DefaultConfig()
	in := int64(1_000_000)
	evs := []Event{
		ev("in", "p", KindDepositIn, 0, in, ""),
		ev("out", "p", KindPaymentOut, time.Hour, in*cfg.RapidRatioBps/10000, "x"),
	}
	if rules(Evaluate(evs, cfg))[RuleRapidInOut] != 1 {
		t.Fatal("paying out exactly the ratio within the window must fire")
	}
	evs[1].AmountMinor--
	if rules(Evaluate(evs, cfg))[RuleRapidInOut] != 0 {
		t.Fatal("one unit under the ratio must not fire")
	}
}

// Parties are monitored separately: one party's pattern must not be attributed to another.
func TestPartiesAreSeparate(t *testing.T) {
	cfg := DefaultConfig()
	evs := []Event{
		ev("1", "a", KindPaymentOut, 0, cfg.VelocityMinor, "x"),
		ev("2", "b", KindPaymentOut, 0, 100, "x"),
	}
	fs := Evaluate(evs, cfg)
	var velocityA bool
	for _, f := range fs {
		if f.PartyID == "b" {
			t.Fatalf("party b was flagged: %+v", f)
		}
		if f.PartyID == "a" && f.Rule == RuleVelocity && strings.Contains(f.Detail, "limit") {
			velocityA = true
		}
	}
	if !velocityA {
		t.Fatalf("party a's velocity must be flagged: %+v", fs)
	}
}
