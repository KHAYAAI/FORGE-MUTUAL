package returns

import (
	"fmt"
	"sort"
	"time"
)

// DI 110, off-balance-sheet activities (monthly), lines 1 to 11. Source: the PA's proposed directive for mutual banks (13 August
// 2025), Annexure F (Government Gazette No. 22738, 5 October 2001); the DI 400 directives (Annexure J2) apply to it. Transcribed by
// reading the scan; a second person must check it.
//
// Columns: 1 month-end balance, 2 maximum level during the month, 3 to 9 the weightings 0%, 5%, 10%, 20%, 50%, 100%, 150%, 10
// impairments. Forge has no guarantees, letters of credit, underwriting, managed portfolios, committed capital expenditure or
// revaluation surplus; it has undrawn credit facilities (line 3) and, once foreign-currency accounts run, an effective net open
// position in foreign currency (line 6, from DI 600 line 8 column 8, weighted 100%).
// Cell references are "DI110.<line>.<column>".

// DI110Lines are the form's line labels.
var DI110Lines = []string{"", "Indemnities and guarantees", "Irrevocable letters of credit", "Unutilized facilities", "Underwriting exposures",
	"Other contingent liabilities", "Effective net open position in foreign currencies", "Portfolios managed: by others on behalf of the reporting institution",
	"Portfolios managed: for others where financing is provided", "Committed capital expenditure",
	"Total of revaluation surplus per regulation 14 not reflected on form DI 100", "TOTAL (of items 1 to 10)"}

// DI110Inputs are the month's off-balance-sheet figures.
type DI110Inputs struct {
	// Irrevocable undrawn facilities by original maturity: under one year weighs 0%, one year and over (to non-banks) 50%.
	UndrawnUnderOneYear, UndrawnOneYearAndOver, MaxUndrawn Cents
	// The effective net open position in foreign currency, in rand (DI 600 line 8 column 8), and its maximum during the month.
	NetOpenFX, MaxNetOpenFX Cents
}

// DI110 computes the return.
func DI110(period string, in DI110Inputs, at time.Time) (Result, error) {
	if in.UndrawnUnderOneYear < 0 || in.UndrawnOneYearAndOver < 0 || in.NetOpenFX < 0 {
		return Result{}, fmt.Errorf("%w: off-balance-sheet amounts are reported as positives", ErrIncomplete)
	}
	r := Result{Form: "DI 110", Period: period, Audited: true, Values: map[string]Cents{}, ComputedAt: at}
	c := func(line, col int) string { return fmt.Sprintf("DI110.%d.%d", line, col) }
	for line := 1; line <= 11; line++ {
		for col := 1; col <= 10; col++ {
			r.Values[c(line, col)] = 0
		}
	}
	r.Values[c(3, 1)] = in.UndrawnUnderOneYear + in.UndrawnOneYearAndOver
	r.Values[c(3, 2)] = in.MaxUndrawn
	r.Values[c(3, 3)] = in.UndrawnUnderOneYear
	r.Values[c(3, 7)] = in.UndrawnOneYearAndOver
	r.Values[c(6, 1)] = in.NetOpenFX
	r.Values[c(6, 2)] = in.MaxNetOpenFX
	r.Values[c(6, 8)] = in.NetOpenFX
	for col := 1; col <= 10; col++ {
		var s Cents
		for line := 1; line <= 10; line++ {
			s += r.Values[c(line, col)]
		}
		r.Values[c(11, col)] = s
	}
	// Each line's weight columns add up to its balance.
	for line := 1; line <= 11; line++ {
		var s Cents
		for col := 3; col <= 10; col++ {
			s += r.Values[c(line, col)]
		}
		if s != r.Values[c(line, 1)] {
			r.Breaks = append(r.Breaks, fmt.Sprintf("DI 110 line %d: weightings add to %d, balance is %d", line, s, r.Values[c(line, 1)]))
		}
	}
	for ref := range r.Values {
		r.Unverified = append(r.Unverified, ref)
	}
	sort.Strings(r.Unverified)
	return r, nil
}
