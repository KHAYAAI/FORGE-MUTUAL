package returns

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// DI 510, large exposures (quarterly, attached to DI 500), and DI 505, the report of a large exposure (whenever required). Source: the
// PA's proposed directive for mutual banks (13 August 2025), Annexures Q1b, Q1c (DI 510: Government Gazette No. 15381, 28 December 1993)
// and Q2 (instructions). Transcribed by reading the scans; a second person must check it.
//
// DI 510 part 1 lists every exposure to a person (a legal entity, not an account) above ten per cent of net qualifying capital and
// reserves (DI 100 line 31 at the latest date): authorisation date, nature, security held, amount granted and utilised, and granted as
// a percentage of capital (line 1 is the total). Part 2 gives the overdue part of those exposures: utilised, overdue, security against
// the overdue amount, specific provisions and the net overdue amount (line 2 is the total). Names go on an attached schedule.
// The exposure to a person adds deposits placed with it, loans and advances, investments, acknowledgements of debt and contingent
// liabilities (DI 100 items 40 to 51 and 63, 70 to 73; DI 110 items 1 to 5) and ten per cent of gross forward exchange contracts.
//
// DI 505 tells the PA (section 51(2) of the Act) of a transaction that takes an exposure to a person above the percentage of capital
// and reserves prescribed in regulation 30(1)(b) (VERIFY the current percentage; it is a parameter here).
//
// Judgement: the sponsor bank and each partner bank are counterparties too: Forge's balances with them (DI 100 line 40) are exposures
// to those banks and will usually be the largest on the return.

// Exposure is the bank's total exposure to one person at the reporting date.
type Exposure struct {
	Person, ReliancePerson string // the person, and the person relied on for repayment
	Authorised             time.Time
	Nature                 string // for example "working-capital loan", "deposit placed with bank"
	Security, Granted      Cents
	Utilised               Cents
	Overdue                Cents
	SecurityOverdue        Cents
	SpecificProvisions     Cents
}

// LargeExposures is a DI 510 with its schedule of persons.
type LargeExposures struct {
	Result
	Rows []Exposure
}

// DI510 computes the return from every exposure and the latest DI 100.
func DI510(period string, exposures []Exposure, di100 Result, at time.Time) (LargeExposures, error) {
	if di100.Form != "DI 100" {
		return LargeExposures{}, fmt.Errorf("%w: DI 510 needs the latest DI 100 (line 31)", ErrIncomplete)
	}
	capital := di100.Values["DI100.31"]
	if capital <= 0 {
		return LargeExposures{}, fmt.Errorf("%w: net qualifying capital (DI 100 line 31) is %d; large exposures cannot be measured against it", ErrIncomplete, capital)
	}
	out := LargeExposures{Result: Result{Form: "DI 510", Period: period, Audited: true, Values: map[string]Cents{}, ComputedAt: at}}
	seen := map[string]bool{}
	for _, e := range exposures {
		key := strings.ToLower(strings.TrimSpace(e.Person))
		if key == "" {
			return LargeExposures{}, fmt.Errorf("%w: an exposure with no person", ErrIncomplete)
		}
		if seen[key] {
			return LargeExposures{}, fmt.Errorf("%w: %s appears twice: aggregate exposures per legal entity before reporting", ErrIncomplete, e.Person)
		}
		seen[key] = true
		if e.Granted*10 > capital {
			out.Rows = append(out.Rows, e)
		}
	}
	sort.Slice(out.Rows, func(i, j int) bool { return out.Rows[i].Granted > out.Rows[j].Granted })
	v := out.Values
	for _, e := range out.Rows {
		v["DI510.1.3"] += e.Security
		v["DI510.1.4"] += e.Granted
		v["DI510.1.5"] += e.Utilised
		if e.Overdue > 0 {
			v["DI510.2.1"] += e.Utilised
			v["DI510.2.2"] += e.Overdue
			v["DI510.2.3"] += e.SecurityOverdue
			v["DI510.2.4"] += e.SpecificProvisions
			v["DI510.2.5"] += e.Overdue - e.SecurityOverdue - e.SpecificProvisions
		}
	}
	v["DI510.1.6"] = v["DI510.1.4"] * 10000 / capital // hundredths of a percent
	for ref := range v {
		out.Unverified = append(out.Unverified, ref)
	}
	sort.Strings(out.Unverified)
	return out, nil
}

// PercentOfCapital is an exposure's granted amount as hundredths of a percent of capital (for DI 510 column 6 per row).
func PercentOfCapital(e Exposure, capital Cents) Cents {
	if capital <= 0 {
		return 0
	}
	return e.Granted * 10000 / capital
}

// DI505Due lists the exposures a transaction has just taken above the prescribed percentage of capital (thresholdBps of DI 100 line
// 31): each needs a DI 505 report. before and after are the person's exposure before and after the transaction.
func DI505Due(before, after map[string]Cents, capital Cents, thresholdBps int64) ([]string, error) {
	if capital <= 0 || thresholdBps <= 0 {
		return nil, fmt.Errorf("%w: DI 505 needs capital and the prescribed percentage", ErrIncomplete)
	}
	limit := capital * Cents(thresholdBps) / 10000
	var due []string
	for person, a := range after {
		if a > limit && before[person] <= limit {
			due = append(due, person)
		}
	}
	sort.Strings(due)
	return due, nil
}
