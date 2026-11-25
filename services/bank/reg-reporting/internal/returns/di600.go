package returns

import (
	"fmt"
	"sort"
	"time"
)

// DI 600, currency risk (monthly), lines 1 to 24. Source: the PA's proposed directive for mutual banks (13 August 2025), Annexures
// R1 (the form, Government Gazette No. 15381, 28 December 1993) and R2 (instructions). Transcribed by reading the scan; a second
// person must check it.
//
// Amounts on this form are in US dollars, so every value here is in US CENTS, not rand. The columns are the 1993 currencies:
// 1 US dollar, 2 pound sterling, 3 German mark, 4 French franc, 5 Swiss franc, 6 Japanese yen, 7 other, 8 total. The mark and the
// franc no longer exist, so the euro goes in column 7 (judgement; VERIFY whether the PA's current template has a euro column).
// Lines 13 to 18 are the maturity ladder for US dollars and 19 to 24 for the other currencies, in nine columns (0-7 days, 8-31
// days, 32 days-3 months, 3-6, 6-9, 9-12 months, 1-3 years, over 3 years, total).
//
// Judgements: every foreign-currency balance the platform holds (customer accounts and partner-bank balances) is repayable on
// demand, so the ladder puts it all in 0-7 days; the aggregate in column 8 of lines 6 and 8 is the larger of total long and total
// short positions (the "shorthand" measure), not their net (VERIFY); the limit on line 9 is applied to the aggregate.
// Cell references are "DI600.<line>.<column>".

// DI600Column places an asset code in a currency column.
var DI600Column = map[string]int{"USD": 1, "GBP": 2, "CHF": 5, "JPY": 6}

// DI600Inputs are the month's foreign-currency figures.
type DI600Inputs struct {
	Positions []Position // every non-ZAR ledger position at month-end, Net in rand cents
	// RandCentsPerUSD is the month-end rate used to convert to dollars, for example 1825 for R18.25 per US$1.
	RandCentsPerUSD Cents
	DI100           Result // the latest DI 100, for line 9 (ten per cent of line 31)
	// Optional: the maximum aggregate open position in the month (line 10, column 8, US cents), the days the limit was exceeded
	// (line 11, column 8) and the board's internal overnight limit (line 12, column 8). Line 10 defaults to the month-end position.
	MaxAggregateUSCents, DaysExceeded, InternalLimitUSCents Cents
}

// DI600 computes the return.
func DI600(period string, in DI600Inputs, at time.Time) (Result, error) {
	if in.RandCentsPerUSD <= 0 {
		return Result{}, fmt.Errorf("%w: DI 600 needs the month-end rand per US dollar rate", ErrIncomplete)
	}
	if in.DI100.Form != "DI 100" {
		return Result{}, fmt.Errorf("%w: DI 600 line 9 needs the latest DI 100", ErrIncomplete)
	}
	r := Result{Form: "DI 600", Period: period, Audited: true, Values: map[string]Cents{}, ComputedAt: at}
	c := func(line, col int) string { return fmt.Sprintf("DI600.%d.%d", line, col) }
	usd := func(rand Cents) Cents { return rand * 100 / in.RandCentsPerUSD }
	for line := 1; line <= 12; line++ {
		for col := 1; col <= 8; col++ {
			r.Values[c(line, col)] = 0
		}
	}
	for line := 13; line <= 24; line++ {
		for col := 1; col <= 9; col++ {
			r.Values[c(line, col)] = 0
		}
	}
	for _, p := range in.Positions {
		if p.Asset == "ZAR" {
			continue
		}
		col, ok := DI600Column[p.Asset]
		if !ok {
			col = 7
		}
		ladderAsset, ladderLiab := 19, 20
		if p.Asset == "USD" {
			ladderAsset, ladderLiab = 13, 14
		}
		if p.Net < 0 {
			r.Values[c(1, col)] += usd(-p.Net)
			r.Values[c(ladderAsset, 1)] += usd(-p.Net)
		} else {
			r.Values[c(2, col)] += usd(p.Net)
			r.Values[c(ladderLiab, 1)] += usd(p.Net)
		}
	}
	var long, short Cents
	for col := 1; col <= 7; col++ {
		open := r.Values[c(1, col)] + r.Values[c(3, col)] - (r.Values[c(2, col)] + r.Values[c(4, col)]) + r.Values[c(5, col)]
		r.Values[c(6, col)] = open
		r.Values[c(8, col)] = open + r.Values[c(7, col)]
		r.Values[c(10, col)] = abs(open)
		if open > 0 {
			long += open
		} else {
			short -= open
		}
		for _, line := range []int{1, 2, 3, 4, 5} {
			r.Values[c(line, 8)] += r.Values[c(line, col)]
		}
	}
	aggregate := max(long, short)
	r.Values[c(6, 8)] = aggregate
	r.Values[c(8, 8)] = aggregate
	r.Values[c(9, 8)] = usd(in.DI100.Values["DI100.31"]) / 10
	r.Values[c(10, 8)] = max(aggregate, in.MaxAggregateUSCents)
	r.Values[c(11, 8)] = in.DaysExceeded
	r.Values[c(12, 8)] = in.InternalLimitUSCents
	for _, base := range []int{13, 19} {
		for col := 1; col <= 8; col++ {
			r.Values[c(base+5, col)] = r.Values[c(base, col)] + r.Values[c(base+2, col)] - (r.Values[c(base+1, col)] + r.Values[c(base+3, col)]) + r.Values[c(base+4, col)]
		}
		for line := base; line <= base+5; line++ {
			var s Cents
			for col := 1; col <= 8; col++ {
				s += r.Values[c(line, col)]
			}
			r.Values[c(line, 9)] = s
		}
	}
	for ref := range r.Values {
		r.Unverified = append(r.Unverified, ref)
	}
	sort.Strings(r.Unverified)
	return r, nil
}

// NetOpenRand is DI 600 line 8 column 8 converted back to rand, for DI 110 line 6.
func NetOpenRand(di600 Result, randCentsPerUSD Cents) Cents {
	return di600.Values["DI600.8.8"] * randCentsPerUSD / 100
}

func abs(c Cents) Cents {
	if c < 0 {
		return -c
	}
	return c
}
