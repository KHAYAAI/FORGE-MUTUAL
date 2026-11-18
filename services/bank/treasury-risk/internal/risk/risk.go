// Package risk evaluates treasury alerts (ARCHITECTURE.md section 13 and 6.2). It is pure: snapshots in, alerts out.
//
// Thresholds are placeholders until treasury and counsel set them (VERIFY). They live in configuration, so changing
// them never changes code.
package risk

import (
	"fmt"
	"math/big"
	"time"
)

// Alert kinds.
const (
	KindConcentration = "CONCENTRATION"
	KindNetOutflow    = "NET_OUTFLOW"
	KindLiquidity     = "LIQUIDITY"
	KindSuspenseAge   = "SUSPENSE_AGE"
)

// Severities.
const (
	SeverityWarning  = "WARNING"
	SeverityCritical = "CRITICAL"
)

// Snapshot is the books of one asset at one moment, reduced to what treasury watches. Amounts are minor units.
type Snapshot struct {
	Asset          string
	At             time.Time
	Deposits       int64  // sum of customer balances (credit-normal)
	LargestDeposit int64  // the largest single customer balance
	LargestAccount string // the account holding it
	Pool           int64  // the bank-side mirror (debit-normal)
	Suspense       int64  // unmatched receipts (credit-normal)
}

// Thresholds are the configured limits. Ratios are basis points (10000 is 100 percent).
type Thresholds struct {
	ConcentrationBps int64
	OutflowBps       int64
	SuspenseMinor    int64
}

// DefaultThresholds are placeholders (VERIFY). They are conservative enough to fire on a real problem and not so
// tight that they fire on ordinary movement.
func DefaultThresholds() Thresholds {
	return Thresholds{ConcentrationBps: 2500, OutflowBps: 2000, SuspenseMinor: 1_000_000}
}

// Alert is one condition that is currently true.
type Alert struct {
	Kind         string
	Asset        string
	Severity     string
	Detail       string
	MetricBps    int64
	ThresholdBps int64
}

// Bps returns part as basis points of whole, rounded down, and never overflows. Zero whole gives zero.
func Bps(part, whole int64) int64 {
	if whole <= 0 {
		return 0
	}
	r := new(big.Int).Mul(big.NewInt(part), big.NewInt(10000))
	r.Quo(r, big.NewInt(whole))
	return r.Int64()
}

// Evaluate returns every alert that holds now. prev is the snapshot from about a day earlier, or nil when there is
// none yet; the outflow rule needs it and does not fire without it.
func Evaluate(cur Snapshot, prev *Snapshot, th Thresholds) []Alert {
	var out []Alert

	if cur.Deposits > 0 {
		if bps := Bps(cur.LargestDeposit, cur.Deposits); bps >= th.ConcentrationBps {
			out = append(out, Alert{
				Kind: KindConcentration, Asset: cur.Asset, Severity: SeverityWarning,
				Detail:       fmt.Sprintf("account %s holds %d bps of deposits, limit %d", cur.LargestAccount, bps, th.ConcentrationBps),
				MetricBps:    bps,
				ThresholdBps: th.ConcentrationBps,
			})
		}
	}

	if prev != nil && prev.Deposits > 0 && prev.Asset == cur.Asset {
		if decline := prev.Deposits - cur.Deposits; decline > 0 {
			if bps := Bps(decline, prev.Deposits); bps >= th.OutflowBps {
				out = append(out, Alert{
					Kind: KindNetOutflow, Asset: cur.Asset, Severity: SeverityWarning,
					Detail:       fmt.Sprintf("deposits fell %d bps in the window, limit %d", bps, th.OutflowBps),
					MetricBps:    bps,
					ThresholdBps: th.OutflowBps,
				})
			}
		}
	}

	if cur.Deposits > cur.Pool {
		out = append(out, Alert{
			Kind: KindLiquidity, Asset: cur.Asset, Severity: SeverityCritical,
			Detail: fmt.Sprintf("deposits %d exceed the pooled balance %d; the books do not reconcile with the bank",
				cur.Deposits, cur.Pool),
		})
	}

	if th.SuspenseMinor > 0 && cur.Suspense > th.SuspenseMinor {
		out = append(out, Alert{
			Kind: KindSuspenseAge, Asset: cur.Asset, Severity: SeverityWarning,
			Detail: fmt.Sprintf("suspense holds %d, limit %d; unmatched receipts need an operator", cur.Suspense, th.SuspenseMinor),
		})
	}
	return out
}
