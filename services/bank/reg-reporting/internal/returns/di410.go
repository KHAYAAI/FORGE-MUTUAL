package returns

import (
	"fmt"
	"sort"
	"time"
)

// DI 410, interest-rate risk (monthly), lines 1 to 25. Source: the PA's proposed directive for mutual banks (13 August 2025), Annexure N
// (Government Gazette No. 15381, 28 December 1993). Transcribed by reading the scan; a second person must check it.
//
// Lines 1 to 16 are the repricing gap: columns 1 total book value, 2 0-31 days, 3 32-91, 4 92-181, 5 182-365, 6 other (including
// non-rate items). Fixed-rate assets and liabilities are lines 1 and 2 (gap 3); variable-rate assets and liabilities, each linked or
// discretionary, are lines 4 to 7 (gap 8); 9 is the subtotal of rated items, 10 non-rate items (capital and other), 11 other balancing
// amounts, 12 the subtotal (its total column is nil when the balance sheet balances), 13 to 15 derivatives, 16 the gap after them.
// Lines 17 to 21 are rate forecasts for 1, 3, 6 and 12 months (basis points). Lines 22 and 23 are the effect on income over the next
// twelve months of a 1% rise and fall; 24 and 25 those as hundredths of a percent of net qualifying capital (DI 100 line 31).
//
// Judgements: every Forge rate is set by Forge and can be changed on notice, so deposits and loans are variable and discretionary
// (lines 5 and 7) and reprice in the bucket of their notice period; the income effect uses the standard gap method, each bucket's gap
// times 1% times the part of the year left after its midpoint (0.5, 2, 4.5 and 9 months; "other" has no effect within the year), and the
// fall is the mirror of the rise (VERIFY the method with the auditor).

// RepricingItem is an amount on a DI 410 line in a repricing column.
type RepricingItem struct {
	Line, Column int // line 1, 2, 4, 5, 6, 7 or 10; column 2 to 6
	Amount       Cents
}

// DI410Inputs are the month's figures.
type DI410Inputs struct {
	DI100     Result
	Items     []RepricingItem
	Forecasts map[int][4]int64 // lines 17 to 21: 1, 3, 6 and 12-month forecasts in basis points
}

var bucketMonthsLeft = map[int]int64{2: 1150, 3: 1000, 4: 750, 5: 300, 6: 0} // hundredths of a month left in the year after the midpoint

// DI410 computes the return.
func DI410(period string, in DI410Inputs, at time.Time) (Result, error) {
	if in.DI100.Form != "DI 100" || in.DI100.Period != period {
		return Result{}, fmt.Errorf("%w: DI 410 needs the DI 100 for %s", ErrIncomplete, period)
	}
	r := Result{Form: "DI 410", Period: period, Audited: true, Values: map[string]Cents{}, ComputedAt: at}
	c := func(line, col int) string { return fmt.Sprintf("DI410.%d.%d", line, col) }
	for line := 1; line <= 16; line++ {
		for col := 1; col <= 6; col++ {
			r.Values[c(line, col)] = 0
		}
	}
	allowed := map[int]bool{1: true, 2: true, 4: true, 5: true, 6: true, 7: true, 10: true}
	for _, it := range in.Items {
		if !allowed[it.Line] || it.Column < 2 || it.Column > 6 {
			return Result{}, fmt.Errorf("%w: line %d column %d is not an input cell", ErrIncomplete, it.Line, it.Column)
		}
		r.Values[c(it.Line, it.Column)] += it.Amount
	}
	for col := 2; col <= 6; col++ {
		g := func(l int) Cents { return r.Values[c(l, col)] }
		r.Values[c(3, col)] = g(1) - g(2)
		r.Values[c(8, col)] = g(4) + g(5) - g(6) - g(7)
		r.Values[c(9, col)] = g(3) + g(8)
		r.Values[c(12, col)] = g(9) + g(10) + g(11)
		r.Values[c(16, col)] = g(12) + g(13) + g(14) + g(15)
	}
	for line := 1; line <= 16; line++ {
		var s Cents
		for col := 2; col <= 6; col++ {
			s += r.Values[c(line, col)]
		}
		r.Values[c(line, 1)] = s
	}
	if r.Values[c(12, 1)] != 0 {
		r.Breaks = append(r.Breaks, fmt.Sprintf("DI 410 line 12 total must be nil (rated and non-rate items balance), is %d", r.Values[c(12, 1)]))
	}
	for line := 17; line <= 21; line++ {
		f := in.Forecasts[line]
		for i := 0; i < 4; i++ {
			r.Values[c(line, i+1)] = Cents(f[i])
		}
	}
	var up Cents
	for col := 2; col <= 6; col++ {
		up += r.Values[c(16, col)] * Cents(bucketMonthsLeft[col]) / 100 / 12 / 100 // gap x 1% x months left / 12
	}
	r.Values["DI410.22"] = up
	r.Values["DI410.23"] = -up
	if capital := in.DI100.Values["DI100.31"]; capital > 0 {
		r.Values["DI410.24"] = up * 10000 / capital
		r.Values["DI410.25"] = -up * 10000 / capital
	}
	for ref := range r.Values {
		r.Unverified = append(r.Unverified, ref)
	}
	sort.Strings(r.Unverified)
	return r, nil
}
