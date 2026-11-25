package returns

import (
	"fmt"
	"sort"
	"time"
)

// DI 700, restriction on investments, loans and advances (annual), lines 1 to 16. Source: the PA's proposed directive for mutual banks
// (13 August 2025), Annexure S (Government Gazette No. 15381, 28 December 1993). Transcribed by reading the scan; a second person must
// check it. Sections 55 and 56 of the Act limit investments in immovable property and shares and loans to certain subsidiaries and
// associates; the DI 099 declaration states they were complied with.
//
// Lines 1 to 3 are investments in immovable property (s.55(1)(a)), shares (s.55(1)(b)) and loans and advances (s.55(1)(c)); 4 their total;
// 5 assets bought in to protect an investment in the last five years; 6 investments as reduced; 7 net qualifying capital (DI 100 line 31);
// 8 the excess of capital over them. Lines 9 to 11 are investments in, loans to and guarantees for certain associates (s.56(1)); 12 their
// total; 13 item 8; 14 the adjusted figure; 15 ten per cent of liabilities to the public (DI 100 line 16); 16 the excess or deficiency.
// Forge holds no property, shares, subsidiaries or associates, so lines 1 to 3 and 9 to 11 are inputs that are zero today.

// DI700Inputs are the year-end figures for the restricted items.
type DI700Inputs struct {
	DI100                                            Result
	Property, Shares, SubsidiaryLoans, BoughtIn      Cents
	AssociateInvestments, AssociateLoans, Guarantees Cents
}

// DI700 computes the return.
func DI700(period string, in DI700Inputs, at time.Time) (Result, error) {
	if in.DI100.Form != "DI 100" || in.DI100.Period != period {
		return Result{}, fmt.Errorf("%w: DI 700 needs the year-end DI 100 for %s", ErrIncomplete, period)
	}
	r := Result{Form: "DI 700", Period: period, Audited: true, Values: map[string]Cents{}, ComputedAt: at}
	v := func(l int) string { return fmt.Sprintf("DI700.%d", l) }
	set := func(l int, c Cents) { r.Values[v(l)] = c }
	set(1, in.Property)
	set(2, in.Shares)
	set(3, in.SubsidiaryLoans)
	set(4, in.Property+in.Shares+in.SubsidiaryLoans)
	set(5, in.BoughtIn)
	set(6, r.Values[v(4)]-in.BoughtIn)
	set(7, in.DI100.Values["DI100.31"])
	set(8, r.Values[v(7)]-r.Values[v(6)])
	set(9, in.AssociateInvestments)
	set(10, in.AssociateLoans)
	set(11, in.Guarantees)
	set(12, in.AssociateInvestments+in.AssociateLoans+in.Guarantees)
	set(13, r.Values[v(8)])
	set(14, r.Values[v(12)]-r.Values[v(13)])
	set(15, in.DI100.Values["DI100.16"]/10)
	set(16, r.Values[v(15)]-r.Values[v(14)])
	for ref := range r.Values {
		r.Unverified = append(r.Unverified, ref)
	}
	sort.Strings(r.Unverified)
	return r, nil
}
