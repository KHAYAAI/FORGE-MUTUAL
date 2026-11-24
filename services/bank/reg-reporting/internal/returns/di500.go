package returns

import (
	"fmt"
	"sort"
	"time"
)

// DI 500, credit risk (quarterly), lines 1 to 27 and paragraphs 6 and 7. Source: the PA's proposed directive for mutual banks (13 August
// 2025), Annexures Q1a (the form, Government Gazette No. 15381, 28 December 1993) and Q2 (instructions). Transcribed by reading the scans
// (page 2 is printed sideways); a second person must check it.
//
// Paragraph 1 (lines 1 to 13): overdue amounts by columns 1 mortgage loans, 2 instalment sales and leases, 3 other loans and advances, 4
// total. Forge offers no mortgages or instalment sales, so its loans are column 3. Line 1 is the months after which the credit policy calls
// an amount overdue. Paragraph 2 (lines 14 to 16): the overdue amounts by age. Paragraph 3 (lines 17 to 23): assets bought in or repossessed
// (none: DI 520 is nil) and other assets (DI 100 item 74). Paragraph 4 (lines 24, 25): assets by profitability, using the instructions'
// rule (a reasonable return earns more than the cost of funds plus operating expenses over total assets; some return earns more than the
// cost of funds; anything else, or anything non-performing, is no return). Paragraph 5 (lines 26, 27, six-monthly): loans by economic
// sector and number of clients. Paragraph 6 lists large exposures rescheduled in the quarter; paragraph 7 the dates the credit procedures
// were last reviewed by the external auditor and the audit committee.
//
// Judgements: the sector total is checked against DI 100 item 52 (the scan reads "item 51", which is the resale line: VERIFY); a borrower's
// sector is the one lending records, using the Standard Industrial Classification groups on the form.

// Sectors are the DI 500 paragraph 5 columns 1 to 11.
var Sectors = []string{"agriculture", "mining", "manufacturing", "property", "construction", "retail trade", "transport", "services", "central treasuries", "individuals", "other"}

// AssetYield is one interest-bearing asset group for paragraph 4.
type AssetYield struct {
	Amount        Cents
	YieldBps      int64
	NonPerforming bool
}

// DI500Inputs are the quarter's credit figures for "other loans and advances" (column 3).
type DI500Inputs struct {
	DI100                                         Result
	PolicyMonths                                  int64 // line 1
	OverdueStart, Recovered, NewOverdue           Cents // lines 2 to 4
	SpecificProvisions, SecurityHeld              Cents // lines 5 and 7
	WrittenOffAgainstProvisions, WrittenOffIncome Cents // lines 10, 11 (financial year to date)
	GeneralProvision                              Cents // line 12, column 4
	UnprovidedSuspended, UnprovidedNotSuspended   Cents // line 13
	Overdue0to3, Overdue3to6, OverdueOver6        Cents // lines 14 to 16
	Yields                                        []AssetYield
	CostOfFundsBps, OpexOverAssetsBps             int64
	SectorAmounts, SectorClients                  map[string]Cents // paragraph 5 (six-monthly), keyed by Sectors
	Rescheduled                                   []string
	ExternalAuditReview, AuditCommitteeReview     time.Time
}

// CreditRisk is a DI 500 with its paragraphs 6 and 7.
type CreditRisk struct {
	Result
	Rescheduled                               []string
	ExternalAuditReview, AuditCommitteeReview time.Time
}

// DI500 computes the return.
func DI500(period string, in DI500Inputs, at time.Time) (CreditRisk, error) {
	if in.DI100.Form != "DI 100" || in.DI100.Period != period {
		return CreditRisk{}, fmt.Errorf("%w: DI 500 needs the DI 100 for %s", ErrIncomplete, period)
	}
	if in.PolicyMonths <= 0 {
		return CreditRisk{}, fmt.Errorf("%w: line 1, when the credit policy calls an amount overdue, is required", ErrIncomplete)
	}
	out := CreditRisk{Result: Result{Form: "DI 500", Period: period, Audited: true, Values: map[string]Cents{}, ComputedAt: at},
		Rescheduled: in.Rescheduled, ExternalAuditReview: in.ExternalAuditReview, AuditCommitteeReview: in.AuditCommitteeReview}
	v := out.Values
	c := func(line, col int) string { return fmt.Sprintf("DI500.%d.%d", line, col) }
	d := in.DI100.Values
	set3 := func(line int, x Cents) { v[c(line, 3)] = x; v[c(line, 4)] = x; v[c(line, 1)] = 0; v[c(line, 2)] = 0 }
	set3(1, Cents(in.PolicyMonths))
	v[c(1, 4)] = 0 // the total column is shaded for line 1
	set3(2, in.OverdueStart)
	set3(3, in.Recovered)
	set3(4, in.NewOverdue)
	set3(5, in.SpecificProvisions)
	set3(6, in.OverdueStart-in.Recovered+in.NewOverdue-in.SpecificProvisions)
	set3(7, in.SecurityHeld)
	set3(8, v[c(6, 3)]-in.SecurityHeld)
	set3(9, in.WrittenOffAgainstProvisions+in.WrittenOffIncome)
	set3(10, in.WrittenOffAgainstProvisions)
	set3(11, in.WrittenOffIncome)
	v[c(12, 4)] = in.GeneralProvision
	v[c(13, 1)], v[c(13, 2)] = in.UnprovidedSuspended, in.UnprovidedNotSuspended
	set3(14, in.Overdue0to3)
	set3(15, in.Overdue3to6)
	set3(16, in.OverdueOver6)
	gross := in.OverdueStart - in.Recovered + in.NewOverdue
	if in.Overdue0to3+in.Overdue3to6+in.OverdueOver6 != gross {
		out.Breaks = append(out.Breaks, fmt.Sprintf("DI 500 lines 14 to 16 add to %d; the gross overdue amount (lines 2 - 3 + 4) is %d", in.Overdue0to3+in.Overdue3to6+in.OverdueOver6, gross))
	}
	// Paragraph 3: nothing bought in; other assets are DI 100 item 74.
	for line := 17; line <= 21; line++ {
		v[c(line, 1)] = 0
	}
	v[c(22, 1)] = d["DI100.74"]
	v[c(23, 1)] = v[c(17, 1)] + v[c(22, 1)]
	// Paragraph 4: profitability.
	v[c(24, 1)] = d["DI100.35"]
	for _, y := range in.Yields {
		switch {
		case y.NonPerforming || y.YieldBps <= in.CostOfFundsBps:
			v[c(24, 4)] += y.Amount
		case y.YieldBps > in.CostOfFundsBps+in.OpexOverAssetsBps:
			v[c(24, 2)] += y.Amount
		default:
			v[c(24, 3)] += y.Amount
		}
	}
	if adv := v[c(24, 2)] + v[c(24, 3)] + v[c(24, 4)]; adv != d["DI100.39"]+d["DI100.55"]+d["DI100.59"] {
		out.Breaks = append(out.Breaks, fmt.Sprintf("DI 500 line 24 columns 2 to 4 add to %d; DI 100 items 39, 55 and 59 are %d", adv, d["DI100.39"]+d["DI100.55"]+d["DI100.59"]))
	}
	v[c(24, 5)], v[c(24, 6)] = 0, d["DI100.56"]+d["DI100.57"]+d["DI100.60"]+d["DI100.61"]
	v[c(24, 7)] = d["DI100.64"]
	v[c(24, 8)] = d["DI100.69"] + d["DI100.74"] + d["DI100.75"]
	v[c(24, 9)] = d["DI100.76"]
	var sum Cents
	for col := 1; col <= 8; col++ {
		sum += v[c(24, col)]
	}
	if sum != v[c(24, 9)] {
		out.Breaks = append(out.Breaks, fmt.Sprintf("DI 500 line 24 columns 1 to 8 add to %d; total assets (DI 100 item 76) are %d", sum, v[c(24, 9)]))
	}
	if total := v[c(24, 9)]; total > 0 {
		for col := 1; col <= 9; col++ {
			v[c(25, col)] = v[c(24, col)] * 10000 / total // hundredths of a percent
		}
	}
	// Paragraph 5: sectors (six-monthly; empty in the other quarters).
	if len(in.SectorAmounts) > 0 {
		var amt, n Cents
		for i, s := range Sectors {
			v[c(26, i+1)] = in.SectorAmounts[s]
			v[c(27, i+1)] = in.SectorClients[s]
			amt += in.SectorAmounts[s]
			n += in.SectorClients[s]
		}
		for s := range in.SectorAmounts {
			if !contains(Sectors, s) {
				return CreditRisk{}, fmt.Errorf("%w: %q is not a sector on the form", ErrIncomplete, s)
			}
		}
		v[c(26, 12)], v[c(27, 12)] = amt, n
		if amt != d["DI100.52"] {
			out.Breaks = append(out.Breaks, fmt.Sprintf("DI 500 sector total %d is not DI 100 item 52 (%d)", amt, d["DI100.52"]))
		}
	}
	for ref := range v {
		out.Unverified = append(out.Unverified, ref)
	}
	sort.Strings(out.Unverified)
	return out, nil
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
