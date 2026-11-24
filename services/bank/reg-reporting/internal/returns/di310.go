package returns

import (
	"fmt"
	"sort"
	"time"
)

// DI 310, minimum reserve balance and liquid assets (monthly), on the form's own lines 1 to 27. Source: the PA's proposed
// directive for mutual banks (13 August 2025), Annexures I1 (the form, Government Gazette No. 22738, 5 October 2001) and I2.
// Transcribed by reading the scan; a second person must check it.
//
// Liquid assets (lines 13 to 19) are only the instruments the Act names: Reserve Bank notes and coin, gold coin and bullion,
// clearing balances with the Reserve Bank, Treasury bills, government securities, Reserve Bank securities and Land Bank bills.
// A balance with a sponsor or partner bank is NOT a liquid asset. The ledger has no account for any of these instruments today,
// so a mutual bank built exactly as the platform stands would report no liquid assets and a shortfall: the business plan's
// balance sheet must hold them (docs/licence-application/BUSINESS_PLAN_DRAFT.md).
//
// Judgement: line 5, "amounts owing by banks and mutual banks", is read as the reporting bank's funding from banks (DI 100 lines 1,
// 10 and 13), which the reserve base excludes (VERIFY with the auditor).

// DI310Inputs are the month's figures.
type DI310Inputs struct {
	DI100 Result // the month-end DI 100
	// The rates: line 7 (the form shows 2.5%) and the liquid-asset ratio of line 11 (the form shows 5%). VERIFY the current figures.
	ReserveRateBps, LiquidRateBps int64
	// Average daily amounts during the month.
	NotesAndCoinForReserve Cents         // line 9
	ReserveBalanceHeld     Cents         // the average daily reserve balance actually held with the Reserve Bank (not a form line)
	Liquid                 map[int]Cents // lines 13 to 19: the average daily amount of each liquid asset class
	RepoWithGovernment     Cents         // line 4
}

// DI310 computes the return.
func DI310(period string, in DI310Inputs, at time.Time) (Result, error) {
	if in.DI100.Form != "DI 100" || in.DI100.Period != period {
		return Result{}, fmt.Errorf("%w: DI 310 needs the DI 100 for %s", ErrIncomplete, period)
	}
	if in.ReserveRateBps <= 0 || in.LiquidRateBps <= 0 {
		return Result{}, fmt.Errorf("%w: the reserve and liquid-asset rates are required", ErrIncomplete)
	}
	r := Result{Form: "DI 310", Period: period, Audited: true, Values: map[string]Cents{}, ComputedAt: at}
	v := func(line int) string { return fmt.Sprintf("DI310.%d", line) }
	d := in.DI100.Values
	r.Values[v(1)] = d["DI100.34"]
	r.Values[v(2)] = d["DI100.23"]
	r.Values[v(3)] = r.Values[v(1)] - r.Values[v(2)]
	r.Values[v(4)] = in.RepoWithGovernment
	r.Values[v(5)] = d["DI100.1"] + d["DI100.10"] + d["DI100.13"]
	r.Values[v(6)] = r.Values[v(3)] - r.Values[v(4)] - r.Values[v(5)]
	r.Values[v(7)] = Cents(in.ReserveRateBps)
	r.Values[v(8)] = r.Values[v(6)] * Cents(in.ReserveRateBps) / 10000
	r.Values[v(9)] = in.NotesAndCoinForReserve
	r.Values[v(10)] = r.Values[v(8)] - r.Values[v(9)]
	r.Values[v(11)] = r.Values[v(3)] * Cents(in.LiquidRateBps) / 10000
	var liquid Cents
	for line := 13; line <= 19; line++ {
		r.Values[v(line)] = in.Liquid[line]
		liquid += in.Liquid[line]
	}
	for line := range in.Liquid {
		if line < 13 || line > 19 {
			return Result{}, fmt.Errorf("%w: line %d is not a liquid-asset line (13 to 19)", ErrIncomplete, line)
		}
	}
	r.Values[v(12)] = liquid
	for line := 20; line <= 26; line++ {
		r.Values[v(line)] = 0 // no cash-management scheme, set-off, resale or repurchase, or vault cash
	}
	var hash Cents
	for line := 1; line <= 26; line++ {
		hash += r.Values[v(line)]
	}
	r.Values[v(27)] = hash // VERIFY how the PA defines the hash total
	r.Values["DI310.x.reserve_held"] = in.ReserveBalanceHeld
	for ref := range r.Values {
		r.Unverified = append(r.Unverified, ref)
	}
	sort.Strings(r.Unverified)
	return r, nil
}

// Shortfalls lists every prescribed minimum a result falls short of, by name, in rand cents. A shortfall is reported, never
// hidden: the DI 099 declaration must be qualified for each.
func Shortfalls(r Result) map[string]Cents {
	out := map[string]Cents{}
	switch r.Form {
	case "DI 400":
		if r.Values["DI400.43"] < 0 {
			out["capital"] = -r.Values["DI400.43"]
		}
	case "DI 600":
		if limit := r.Values["DI600.9.8"]; r.Values["DI600.8.8"] > limit {
			out["effective net open foreign-currency position (US cents)"] = r.Values["DI600.8.8"] - limit
		}
		if r.Values["DI600.11.8"] > 0 && len(out) == 0 {
			out["days the open-position limit was exceeded"] = r.Values["DI600.11.8"]
		}
	case "DI 310":
		if gap := r.Values["DI310.11"] - r.Values["DI310.12"]; gap > 0 {
			out["liquid assets"] = gap
		}
		if gap := r.Values["DI310.10"] - r.Values["DI310.x.reserve_held"]; gap > 0 {
			out["minimum reserve balance"] = gap
		}
	}
	return out
}
