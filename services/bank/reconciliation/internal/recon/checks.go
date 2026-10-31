// Package recon holds the reconciliation arithmetic (ARCHITECTURE.md section 13). It is pure: no I/O, no clocks.
//
// Sign conventions. Each account code has a natural side. Liability codes (customer deposits, fees, suspense,
// operating, and money owed to the sponsor for outbound payments) are credit-normal: their balance is credits minus
// debits. The pool code mirrors the bank's pooled account and is debit-normal: its balance is debits minus credits.
// Totals are kept as debits minus credits for every code, so double entry is a single sum that must be zero.
package recon

import (
	"errors"
	"fmt"
	"sort"
)

// Rail describes one monitored bank relationship: its ledger, the pool account that mirrors the bank, and which
// codes count as Forge's liabilities.
type Rail struct {
	Asset          string
	Ledger         uint32
	Source         string // "sponsor" or "partner"
	PoolCode       uint16
	LiabilityCodes []uint16
}

// ZAR is the sponsor pooled account (Appendix B, code 3003) for ZAR on ledger 710.
var ZAR = Rail{
	Asset: "ZAR", Ledger: 710, Source: "sponsor", PoolCode: 3003,
	LiabilityCodes: []uint16{1001, 2001, 2002, 2005, 3001},
}

// USD is the partner-bank mirror (Appendix B, code 5001) for USD on ledger 840.
var USD = Rail{
	Asset: "USD", Ledger: 840, Source: "partner", PoolCode: 5001,
	LiabilityCodes: []uint16{1001, 2001, 2002, 2005},
}

// Rails are the monitored relationships in v1.
var Rails = []Rail{ZAR, USD}

// ByAsset returns the rail for an asset.
func ByAsset(asset string) (Rail, bool) {
	for _, r := range Rails {
		if r.Asset == asset {
			return r, true
		}
	}
	return Rail{}, false
}

// Totals holds, per account code, the sum of debits minus credits over every account of one ledger.
type Totals struct {
	DebitsMinusCredits map[uint16]int64
	Accounts           map[uint16]int
}

// NewTotals returns an empty Totals.
func NewTotals() Totals {
	return Totals{DebitsMinusCredits: map[uint16]int64{}, Accounts: map[uint16]int{}}
}

// Add records one account's posted debits and credits under its code. Posted balances are never negative, so a
// negative input is refused. The running sum is checked for overflow, because money is never silently wrapped.
func (t *Totals) Add(code uint16, debitsPosted, creditsPosted int64) error {
	if debitsPosted < 0 || creditsPosted < 0 {
		return errors.New("recon: posted debits and credits must not be negative")
	}
	raw := debitsPosted - creditsPosted
	prev := t.DebitsMinusCredits[code]
	sum := prev + raw
	if (raw > 0 && sum < prev) || (raw < 0 && sum > prev) {
		return fmt.Errorf("recon: sum overflow for code %d", code)
	}
	t.DebitsMinusCredits[code] = sum
	t.Accounts[code]++
	return nil
}

// Statement is the bank's view of one day for one asset, reduced to the figures the checks use.
type Statement struct {
	Opening     int64
	Closing     int64
	InboundSum  int64 // sum of IN movements
	OutboundSum int64 // sum of OUT movements, positive
}

// InFlight is money that has moved at one side but not yet at the other. Inbound is credited by the bank and not yet
// booked in the ledger. Outbound is debited by the bank and not yet booked in the ledger.
type InFlight struct {
	Inbound  int64
	Outbound int64
}

// Result is one check's outcome. Difference is left-hand side minus right-hand side, in minor units; zero matches.
type Result struct {
	Check      string
	Matched    bool
	Difference int64
	Detail     string
}

// Check names. Equation and mirror checks are prefixed with the source name.
const (
	CheckStatementArithmetic = "statement_arithmetic"
	CheckLedgerDoubleEntry   = "ledger_double_entry"
	CheckUnclassifiedBalance = "unclassified_balance"
)

// Evaluate runs every check for one rail. It never returns a partial set: every rail gets every check.
func Evaluate(r Rail, t Totals, s Statement, f InFlight) []Result {
	liab := map[uint16]bool{}
	for _, c := range r.LiabilityCodes {
		liab[c] = true
	}

	var results []Result

	arith := s.Closing - (s.Opening + s.InboundSum - s.OutboundSum)
	results = append(results, Result{
		Check: CheckStatementArithmetic, Matched: arith == 0, Difference: arith,
		Detail: "closing must equal opening plus inbound minus outbound",
	})

	var all int64
	for _, v := range t.DebitsMinusCredits {
		all += v
	}
	results = append(results, Result{
		Check: CheckLedgerDoubleEntry, Matched: all == 0, Difference: all,
		Detail: "debits minus credits across every account of the ledger must be zero",
	})

	var unclassified int64
	var unknownCodes []uint16
	for code, v := range t.DebitsMinusCredits {
		if code == r.PoolCode || liab[code] || v == 0 {
			continue
		}
		unknownCodes = append(unknownCodes, code)
		if v < 0 {
			unclassified -= v
		} else {
			unclassified += v
		}
	}
	sort.Slice(unknownCodes, func(i, j int) bool { return unknownCodes[i] < unknownCodes[j] })
	unclassDetail := "every non-zero account must carry a classified code"
	if len(unknownCodes) > 0 {
		unclassDetail = fmt.Sprintf("non-zero balances on unclassified codes %v", unknownCodes)
	}
	results = append(results, Result{
		Check: CheckUnclassifiedBalance, Matched: unclassified == 0, Difference: unclassified, Detail: unclassDetail,
	})

	var liabilities int64
	for _, c := range r.LiabilityCodes {
		liabilities -= t.DebitsMinusCredits[c]
	}
	expected := liabilities + f.Inbound - f.Outbound
	eq := s.Closing - expected
	results = append(results, Result{
		Check:      r.Source + "_equation",
		Matched:    eq == 0,
		Difference: eq,
		Detail: fmt.Sprintf("statement closing %d must equal liabilities %d plus in-flight inbound %d minus in-flight outbound %d",
			s.Closing, liabilities, f.Inbound, f.Outbound),
	})

	pool := t.DebitsMinusCredits[r.PoolCode]
	mirrorExpected := s.Closing - f.Inbound + f.Outbound
	mir := pool - mirrorExpected
	results = append(results, Result{
		Check:      r.Source + "_pool_mirror",
		Matched:    mir == 0,
		Difference: mir,
		Detail: fmt.Sprintf("pool mirror (code %d) %d must equal statement closing %d adjusted for in-flight",
			r.PoolCode, pool, s.Closing),
	})
	return results
}

// AllMatched reports whether every result matched.
func AllMatched(results []Result) bool {
	for _, r := range results {
		if !r.Matched {
			return false
		}
	}
	return len(results) > 0
}
