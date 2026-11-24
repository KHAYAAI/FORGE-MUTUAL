package returns

import (
	"fmt"
	"sort"
	"time"
)

// DI 400, capital adequacy (quarterly), on the form's own lines. Source: the PA's proposed directive for mutual banks
// (13 August 2025), Annexures J1 (the form, Government Gazette No. 22738, 5 October 2001) and J2 (instructions and the table of
// risk weightings). Transcribed by reading the scans; a second person must check it.
//
// Division I weights the quarter's average assets (DI 100) and off-balance-sheet activities (DI 110) into the categories 0%, 5%,
// 10%, 20%, 50%, 100% and 150%, plus impairments, and applies the capital ratio. Division II builds qualifying capital: under the
// form's footnote (a), profit not yet moved to a reserve by formal board resolution does NOT count, so Forge's profit on DI 100
// line 32 is excluded until the board appropriates it. Division III compares capital with the requirement in the last return;
// Division IV is the risk-asset ratio; Division V is unrealised gains on investments (none: Forge holds no investment portfolio).
//
// Cell references: "DI400.<line>.<column>" for Division I (columns 1 to 9: 0%, 5%, 10%, 20%, 50%, 100%, 150%, impairments, total)
// and "DI400.<line>" for single-value lines. Line 44 is in hundredths of a percent (1234 means 12.34%).

// RiskColumns are Division I's weight columns, in basis points of weighting, column 1 to 7.
var RiskColumns = []int64{0, 500, 1000, 2000, 5000, 10000, 15000}

// DI100Weight is the column (1 to 8) each DI 100 asset line falls in under the J2 table. Judgements (the table cannot see the
// counterparty): balances with banks are "banks in RSA and OECD countries" (20%), which holds for the sponsor and the partner
// banks only if each is South African or in the OECD (VERIFY per partner); mortgage loans take 100% because the 50% category
// needs conditions the platform does not check; deferred tax is 0% and must move to impairments if it arises from an assessed loss.
var DI100Weight = map[int]int{
	36: 1, 37: 1, 38: 1, // money: 0%
	40: 4, 41: 4, 50: 4, // banks: 20%
	42: 6, 43: 6, 44: 6, 45: 6, 46: 6, 47: 6, 48: 6, 51: 6, // other loans: 100%
	55: 6, 56: 6, 57: 6, 59: 6, 60: 6, 61: 6, // portfolios: 100% (none held)
	65: 6, 66: 6, 67: 6, 68: 6, // fixed assets: 100%
	70: 6, 71: 6, 72: 6, 73: 6, 74: 6, // per contra and other assets: 100%
	75: 1, // deferred tax not from an assessed loss: 0%
}

// DI400Inputs are what the capital return needs.
type DI400Inputs struct {
	// AverageAssets are the quarter's average DI 100 asset leaf lines (line number to rand cents), net of specific provisions.
	AverageAssets map[int]Cents
	// Undrawn irrevocable facilities, from lending: under one year original maturity take 0%; one year and over to non-banks take 50%.
	UndrawnUnderOneYear, UndrawnOneYearAndOver Cents
	// CapitalRatioBps is line 10 (the form shows 10%, VERIFY the current figure); PhaseInBps is line 12 (10000 = 100%).
	CapitalRatioBps, PhaseInBps int64
	// Capital at quarter end: member shares (line 16), reserves the board has appropriated by formal resolution (line 20), general
	// debt provisions (line 34) and any secondary capital (lines 30, 31).
	MemberShares, AppropriatedReserves, GeneralProvisions, SecondaryShares, SecondaryDebt Cents
	// PreviousRequired is line 13 of the last DI 400 (line 42 here); zero for the first return.
	PreviousRequired Cents
}

// CapitalShortfall is how far capital is below the requirement (DI 400 line 43), or zero. A shortfall is reported, not hidden:
// the return can still be prepared, and the DI 099 declaration refuses to go out unless it is qualified with the statement of
// the deficiency.
func CapitalShortfall(r Result) Cents { return Shortfalls(r)["capital"] }

// DI400 computes the return. It refuses an average-asset line it does not know how to weight.
func DI400(period string, in DI400Inputs, at time.Time) (Result, error) {
	r := Result{Form: "DI 400", Period: period, Audited: true, Values: map[string]Cents{}, ComputedAt: at}
	cell := func(line, col int) string { return fmt.Sprintf("DI400.%d.%d", line, col) }
	set := func(ref string, v Cents) { r.Values[ref] = v }
	lines := make([]int, 0, len(in.AverageAssets))
	for l := range in.AverageAssets {
		lines = append(lines, l)
	}
	sort.Ints(lines)
	for _, l := range lines {
		col, ok := DI100Weight[l]
		if !ok {
			return Result{}, fmt.Errorf("%w: DI 100 line %d has no risk weighting", ErrIncomplete, l)
		}
		r.Values[cell(1, col)] += in.AverageAssets[l]
	}
	set(cell(2, 1), in.UndrawnUnderOneYear) // all irrevocable unutilised facilities under one year: 0%
	set(cell(2, 5), in.UndrawnOneYearAndOver)
	var weighted, total Cents
	for col := 1; col <= 8; col++ {
		v3 := r.Values[cell(1, col)] + r.Values[cell(2, col)]
		set(cell(3, col), v3)
		set(cell(4, col), 0) // no guarantees or pledges received
		set(cell(5, col), 0)
		set(cell(6, col), 0)
		set(cell(7, col), v3)
		if col <= 7 {
			w := v3 * Cents(RiskColumns[col-1]) / 10000
			set(cell(9, col), w)
			weighted += w
		}
		total += v3
	}
	for _, line := range []int{1, 2, 3, 4, 5, 6, 7} {
		var s Cents
		for col := 1; col <= 8; col++ {
			s += r.Values[cell(line, col)]
		}
		set(cell(line, 9), s)
	}
	set(cell(9, 9), weighted)
	if in.CapitalRatioBps <= 0 || in.PhaseInBps <= 0 {
		return Result{}, fmt.Errorf("%w: the capital ratio (line 10) and phase-in (line 12) are required", ErrIncomplete)
	}
	set("DI400.10", Cents(in.CapitalRatioBps))
	before := weighted * Cents(in.CapitalRatioBps) / 10000
	set("DI400.11", before)
	set("DI400.12", Cents(in.PhaseInBps))
	required := before * Cents(in.PhaseInBps) / 10000
	set("DI400.13", required)

	// Division II
	set("DI400.16", in.MemberShares)
	set("DI400.17", 0)
	set("DI400.15", in.MemberShares)
	set("DI400.19", 0)
	set("DI400.20", in.AppropriatedReserves)
	set("DI400.21", 0)
	for l := 23; l <= 27; l++ {
		set(fmt.Sprintf("DI400.%d", l), 0)
	}
	set("DI400.22", 0)
	set("DI400.18", in.AppropriatedReserves)
	primary := in.MemberShares + in.AppropriatedReserves
	set("DI400.14", primary)
	set("DI400.30", in.SecondaryShares)
	set("DI400.31", in.SecondaryDebt)
	set("DI400.29", in.SecondaryShares+in.SecondaryDebt)
	set("DI400.33", 0)
	set("DI400.34", in.GeneralProvisions)
	set("DI400.35", 0)
	set("DI400.32", in.GeneralProvisions)
	secondary := in.SecondaryShares + in.SecondaryDebt + in.GeneralProvisions
	set("DI400.28", secondary)
	qualifyingSecondary := min(secondary, primary) // may not exceed item 14
	set("DI400.36", qualifyingSecondary)
	set("DI400.37", primary+qualifyingSecondary)
	set("DI400.38", r.Values[cell(7, 8)])
	net := primary + qualifyingSecondary - r.Values[cell(7, 8)]
	set("DI400.39", net)
	set("DI400.40", secondary-qualifyingSecondary) // item 28 less item 36 plus 50% of revaluation surpluses (none)
	// Division III
	set("DI400.41", net)
	set("DI400.42", in.PreviousRequired)
	set("DI400.43", net-in.PreviousRequired)
	// Division IV: hundredths of a percent, rounded down
	if weighted > 0 {
		set("DI400.44", net*10000/weighted)
	}
	// Division V
	set("DI400.45", 0)
	for ref := range r.Values {
		r.Unverified = append(r.Unverified, ref)
	}
	sort.Strings(r.Unverified)
	return r, nil
}
