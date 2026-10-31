// Package monitor evaluates transaction-monitoring rules over a party's events (ARCHITECTURE.md 15.1). It is pure:
// events in, findings out. It only sees the events it is given, so a rule that compares against history is only as
// good as the history supplied.
//
// Thresholds are placeholders (VERIFY). Each rule is a scenario in the tests, so a change to a rule shows up as a
// test failure before it reaches a party.
package monitor

import (
	"fmt"
	"sort"
	"time"
)

// Event kinds.
const (
	KindPaymentOut = "PAYMENT_OUT"
	KindDepositIn  = "DEPOSIT_IN"
)

// Rules.
const (
	RuleVelocity       = "VELOCITY"
	RuleStructuring    = "STRUCTURING"
	RuleNewBeneficiary = "NEW_BENEFICIARY_LARGE"
	RuleRapidInOut     = "RAPID_IN_OUT"
)

// Event is one money movement for one party.
type Event struct {
	ID          string
	PartyID     string
	Kind        string
	At          time.Time
	AmountMinor int64
	Beneficiary string // for PAYMENT_OUT
}

// Config holds the rule thresholds.
type Config struct {
	VelocityWindow           time.Duration
	VelocityMinor            int64
	StructuringLowMinor      int64 // inclusive lower bound of the band
	StructuringHighMinor     int64 // exclusive upper bound of the band
	StructuringCount         int
	StructuringWindow        time.Duration
	LargeNewBeneficiaryMinor int64
	RapidWindow              time.Duration
	RapidRatioBps            int64
}

// DefaultConfig holds placeholder thresholds (VERIFY).
func DefaultConfig() Config {
	return Config{
		VelocityWindow:           24 * time.Hour,
		VelocityMinor:            5_000_000, // R50,000
		StructuringLowMinor:      4_500_000, // just under R50,000
		StructuringHighMinor:     5_000_000,
		StructuringCount:         3,
		StructuringWindow:        7 * 24 * time.Hour,
		LargeNewBeneficiaryMinor: 2_000_000, // R20,000
		RapidWindow:              24 * time.Hour,
		RapidRatioBps:            9000,
	}
}

// Finding is one rule that fired for one party.
type Finding struct {
	Rule     string
	PartyID  string
	Severity string
	Detail   string
	Evidence []string
}

// Evaluate runs every rule over the events, grouped by party.
func Evaluate(events []Event, cfg Config) []Finding {
	byParty := map[string][]Event{}
	for _, e := range events {
		byParty[e.PartyID] = append(byParty[e.PartyID], e)
	}
	parties := make([]string, 0, len(byParty))
	for p := range byParty {
		parties = append(parties, p)
	}
	sort.Strings(parties)

	var out []Finding
	for _, p := range parties {
		evs := byParty[p]
		sort.SliceStable(evs, func(i, j int) bool { return evs[i].At.Before(evs[j].At) })
		out = append(out, velocity(p, evs, cfg)...)
		out = append(out, structuring(p, evs, cfg)...)
		out = append(out, newBeneficiary(p, evs, cfg)...)
		out = append(out, rapidInOut(p, evs, cfg)...)
	}
	return out
}

func velocity(party string, evs []Event, cfg Config) []Finding {
	for i, anchor := range evs {
		if anchor.Kind != KindPaymentOut {
			continue
		}
		var sum int64
		var ids []string
		for _, e := range evs[:i+1] {
			if e.Kind == KindPaymentOut && anchor.At.Sub(e.At) <= cfg.VelocityWindow {
				sum += e.AmountMinor
				ids = append(ids, e.ID)
			}
		}
		if sum >= cfg.VelocityMinor {
			return []Finding{{
				Rule: RuleVelocity, PartyID: party, Severity: "WARNING", Evidence: ids,
				Detail: fmt.Sprintf("%d minor units paid out in %s, limit %d", sum, cfg.VelocityWindow, cfg.VelocityMinor),
			}}
		}
	}
	return nil
}

func structuring(party string, evs []Event, cfg Config) []Finding {
	for i, anchor := range evs {
		if anchor.Kind != KindPaymentOut {
			continue
		}
		var ids []string
		for _, e := range evs[:i+1] {
			inBand := e.AmountMinor >= cfg.StructuringLowMinor && e.AmountMinor < cfg.StructuringHighMinor
			if e.Kind == KindPaymentOut && inBand && anchor.At.Sub(e.At) <= cfg.StructuringWindow {
				ids = append(ids, e.ID)
			}
		}
		if len(ids) >= cfg.StructuringCount {
			return []Finding{{
				Rule: RuleStructuring, PartyID: party, Severity: "WARNING", Evidence: ids,
				Detail: fmt.Sprintf("%d payments just under the reporting band in %s", len(ids), cfg.StructuringWindow),
			}}
		}
	}
	return nil
}

// newBeneficiary fires on a large payment to a beneficiary not paid earlier in the supplied history.
func newBeneficiary(party string, evs []Event, cfg Config) []Finding {
	seen := map[string]bool{}
	var out []Finding
	for _, e := range evs {
		if e.Kind != KindPaymentOut {
			continue
		}
		first := !seen[e.Beneficiary]
		seen[e.Beneficiary] = true
		if first && e.AmountMinor >= cfg.LargeNewBeneficiaryMinor {
			out = append(out, Finding{
				Rule: RuleNewBeneficiary, PartyID: party, Severity: "WARNING", Evidence: []string{e.ID},
				Detail: fmt.Sprintf("first payment to beneficiary %q of %d minor units", e.Beneficiary, e.AmountMinor),
			})
		}
	}
	return out
}

// rapidInOut fires when money in is largely paid out again within the window: a common pattern for a mule account.
func rapidInOut(party string, evs []Event, cfg Config) []Finding {
	var out []Finding
	for _, in := range evs {
		if in.Kind != KindDepositIn || in.AmountMinor <= 0 {
			continue
		}
		var outSum int64
		ids := []string{in.ID}
		for _, e := range evs {
			if e.Kind == KindPaymentOut && !e.At.Before(in.At) && e.At.Sub(in.At) <= cfg.RapidWindow {
				outSum += e.AmountMinor
				ids = append(ids, e.ID)
			}
		}
		if outSum*10000 >= in.AmountMinor*cfg.RapidRatioBps {
			out = append(out, Finding{
				Rule: RuleRapidInOut, PartyID: party, Severity: "WARNING", Evidence: ids,
				Detail: fmt.Sprintf("%d paid out within %s of a deposit of %d", outSum, cfg.RapidWindow, in.AmountMinor),
			})
		}
	}
	return out
}
