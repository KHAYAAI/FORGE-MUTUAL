package returns

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// DI 300, liquidity risk: maturity ladder (monthly), lines 1 to 36. Source: the PA's proposed directive for mutual banks (13 August
// 2025), Annexures H1 (the form, Government Gazette No. 15381, 28 December 1993) and H2 (which maps lines 1 to 13 to DI 100). Transcribed
// by reading the scans; a second person must check it.
//
// Columns: 1 month-end balance (total), 2 0-31 days, 3 32-60 days, 4 61-91 days, 5 92-181 days, 6 182-365 days, 7 other.
// Lines 1 to 13 are DI 100 lines 35, 39, 63, 64, 69, 74+75 (assets) and 1, 4, 9, 12, 15+22, 17, 23 (liabilities), each spread over
// the maturity columns; line 14 is the on-balance-sheet mismatch, 17 and 18 the theoretical and cumulative mismatch, 31 to 33 deposit
// concentration. Cell references are "DI300.<line>.<column>".
//
// Contractual maturities: money and balances with banks are 0-31 days; current and savings deposits are repayable on demand (0-31
// days); notice deposits fall in the column of their notice period; loans follow their repayment schedule, which lending must
// supply (there is no default); fixed and other assets, other liabilities and capital are "other". Judgements: wholesale deposits
// (line 31) are every deposit product except those whose id starts with "personal-" (VERIFY the PA's definition); line 19 (the ALCO
// behavioural model) is not built.

// DI300Inputs are the month's figures.
type DI300Inputs struct {
	DI100     Result
	Positions []Position // the same month-end positions the DI 100 was built from
	// LoanMaturities spread DI 300 line 2 (loans and advances) over columns 2 to 7, from the lending schedules. They must add up to
	// DI 100 line 39.
	LoanMaturities [6]Cents
	// Anticipated future cash flows (lines 15, 16) per column 2 to 7, and the ten largest depositors' total (line 33, column 1).
	Inward, Outward      [6]Cents
	TenLargestDepositors Cents
}

// noticeColumn is the DI 300 column for a deposit product.
func noticeColumn(product string) int {
	days := 0
	if strings.HasPrefix(product, "notice-") {
		fmt.Sscanf(strings.TrimPrefix(product, "notice-"), "%d", &days)
	}
	switch {
	case days <= 31:
		return 2
	case days <= 60:
		return 3
	case days <= 91:
		return 4
	case days <= 181:
		return 5
	case days <= 365:
		return 6
	}
	return 7
}

// DI300 computes the return.
func DI300(period string, in DI300Inputs, at time.Time) (Result, error) {
	if in.DI100.Form != "DI 100" || in.DI100.Period != period {
		return Result{}, fmt.Errorf("%w: DI 300 needs the DI 100 for %s", ErrIncomplete, period)
	}
	r := Result{Form: "DI 300", Period: period, Audited: true, Values: map[string]Cents{}, ComputedAt: at}
	c := func(line, col int) string { return fmt.Sprintf("DI300.%d.%d", line, col) }
	d := in.DI100.Values
	for line := 1; line <= 36; line++ {
		for col := 1; col <= 7; col++ {
			r.Values[c(line, col)] = 0
		}
	}
	totals := map[int]Cents{
		1: d["DI100.35"], 2: d["DI100.39"], 3: d["DI100.63"], 4: d["DI100.64"], 5: d["DI100.69"], 6: d["DI100.74"] + d["DI100.75"],
		7: d["DI100.1"], 8: d["DI100.4"], 9: d["DI100.9"], 10: d["DI100.12"], 11: d["DI100.15"] + d["DI100.22"], 12: d["DI100.17"], 13: d["DI100.23"],
	}
	for line, v := range totals {
		r.Values[c(line, 1)] = v
	}
	// Assets: money is short-term; loans follow the schedule; the rest is "other". Balances with banks are in line 2 (DI 100 line
	// 39 includes line 40), so the loan schedule must include them in 0-31 days.
	r.Values[c(1, 2)] = totals[1]
	var sched Cents
	for i, v := range in.LoanMaturities {
		r.Values[c(2, i+2)] = v
		sched += v
	}
	if sched != totals[2] {
		return Result{}, fmt.Errorf("%w: the loan maturities add to %d, DI 100 line 39 is %d (balances with banks go in 0-31 days)", ErrIncomplete, sched, totals[2])
	}
	for _, line := range []int{3, 4, 5, 6} {
		r.Values[c(line, 7)] = totals[line]
	}
	// Liabilities: rand non-bank deposits by product; foreign and interbank funding on demand; the rest "other".
	var wholesale, retail Cents
	for _, p := range in.Positions {
		if p.Code != 1001 || p.Asset != "ZAR" {
			continue
		}
		r.Values[c(8, noticeColumn(p.Product))] += p.Net
		if strings.HasPrefix(p.Product, "personal-") {
			retail += p.Net
		} else {
			wholesale += p.Net
		}
	}
	r.Values[c(7, 2)] = totals[7]
	r.Values[c(9, 2)] = totals[9]
	r.Values[c(10, 2)] = totals[10]
	for _, line := range []int{11, 12, 13} {
		r.Values[c(line, 7)] = totals[line]
	}
	for line := 1; line <= 13; line++ {
		var s Cents
		for col := 2; col <= 7; col++ {
			s += r.Values[c(line, col)]
		}
		if s != totals[line] {
			r.Breaks = append(r.Breaks, fmt.Sprintf("DI 300 line %d: the maturity columns add to %d, the balance is %d", line, s, totals[line]))
		}
	}
	var cumulative Cents
	for col := 1; col <= 7; col++ {
		var mismatch Cents
		for line := 1; line <= 6; line++ {
			mismatch += r.Values[c(line, col)]
		}
		for line := 7; line <= 13; line++ {
			mismatch -= r.Values[c(line, col)]
		}
		r.Values[c(14, col)] = mismatch // column 1 is nil when the balance sheet balances
		if col >= 2 {
			r.Values[c(15, col)] = in.Inward[col-2]
			r.Values[c(16, col)] = in.Outward[col-2]
			r.Values[c(17, col)] = mismatch + in.Inward[col-2] - in.Outward[col-2]
			cumulative += r.Values[c(17, col)]
			r.Values[c(18, col)] = cumulative
		}
	}
	if r.Values[c(14, 1)] != 0 {
		r.Breaks = append(r.Breaks, fmt.Sprintf("DI 300 line 14 column 1 must be nil, is %d", r.Values[c(14, 1)]))
	}
	// Funding available: deposits held at banks (DI 100 line 40) are available within 0-31 days.
	r.Values[c(28, 1)] = d["DI100.40"]
	r.Values[c(28, 2)] = d["DI100.40"]
	wholesale += totals[7] // interbank funding is wholesale
	r.Values[c(31, 1)] = wholesale
	r.Values[c(32, 1)] = retail
	r.Values[c(33, 1)] = in.TenLargestDepositors
	if wholesale+retail != totals[7]+totals[8] { // the form: items 31 and 32 = items 7 and 8
		r.Breaks = append(r.Breaks, fmt.Sprintf("DI 300 lines 31 and 32 add to %d, lines 7 and 8 to %d", wholesale+retail, totals[7]+totals[8]))
	}
	for ref := range r.Values {
		r.Unverified = append(r.Unverified, ref)
	}
	sort.Strings(r.Unverified)
	return r, nil
}
