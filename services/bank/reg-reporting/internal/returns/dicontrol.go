package returns

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// The DI returns that are not computed line by line: forms that apply to a mutual bank but on which Forge has nothing to report
// (nil returns), forms marked not applicable to a mutual bank operating only in the Republic, and the DI 099 control sheet that
// goes with every set. Source: the PA's proposed directive for mutual banks (13 August 2025), Annexures A, B and C
// (docs/regulatory/returns/catalogue.csv says how each of the 27 DI forms is treated).

// NilForm is a form Forge files as nil, and why. Attest is true when a person must confirm the nil, because the ledger alone
// cannot prove it.
type NilForm struct {
	Title, Reason string
	Attest        bool
}

// NilForms are the DI forms that apply but have nothing in them for Forge. Each reason holds because the ledger has no
// account code for that kind of asset or activity: if one appears, the chart check below fails and the nil cannot be declared.
var NilForms = map[string]NilForm{
	"DI 420": {"Market risk (position risk)", "no trading book: the ledger has no trading account code", false},
	"DI 430": {"Trading risk", "no trading book: the ledger has no trading account code", false},
	"DI 520": {"Assets bought in", "no asset bought in to protect an advance: the ledger has no code for one", false},
	"DI 525": {"Funding of local governments", "no loan to a local government; the ledger cannot show the borrower's type, so a person confirms it", true},
	"DI 701": {"Asset-backed securitisation", "no securitisation: the ledger has no code for one", false},
	"DI 702": {"Return regarding investments and interests held", "no investments or interests held: the ledger has no investment code", false},
	"DI 910": {"Institutional breakdown of issuers of and transactions in selected assets", "no securities held: the ledger has no securities code", false},
	"DI 920": {"Analysis of instalment sale and leasing transactions", "no instalment sale or leasing product: the ledger has no code for one", false},
}

// NotApplicable are DI forms marked not applicable to a mutual bank in the Republic, or that need foreign operations Forge does
// not have.
var NotApplicable = map[string]string{
	"DI 099A": "no off-shore subsidiary, branch or associate",
	"DI 401":  "marked N/A for a mutual bank in the Republic (Annexure A); Forge has no subsidiaries",
	"DI 403":  "no foreign operations",
}

var (
	ErrNotNil      = errors.New("returns: the form cannot be declared nil")
	ErrNeedsPerson = errors.New("returns: a named person must confirm this")
)

// NilReturn is a filed nil return.
type NilReturn struct {
	Form, Period, Reason string
	AttestedBy           string
	At                   time.Time
}

// Nil declares a nil return for a form. It refuses if any ledger position has no known place on DI 100 (an unknown code could be
// exactly the kind of asset the nil denies), and, where the form needs it, if no person confirms it.
func Nil(form, period string, ps []Position, attestedBy string, at time.Time) (NilReturn, error) {
	nf, ok := NilForms[form]
	if !ok {
		return NilReturn{}, fmt.Errorf("%w: %s is not a nil form", ErrNotNil, form)
	}
	for _, p := range ps {
		if _, _, err := mapDI100(p); err != nil {
			return NilReturn{}, fmt.Errorf("%w: %s: %v", ErrNotNil, form, err)
		}
	}
	attestedBy = strings.TrimSpace(attestedBy)
	if nf.Attest && attestedBy == "" {
		return NilReturn{}, fmt.Errorf("%w: %s nil return (%s)", ErrNeedsPerson, form, nf.Reason)
	}
	return NilReturn{Form: form, Period: period, Reason: nf.Reason, AttestedBy: attestedBy, At: at}, nil
}

// Declaration is the DI 099 control sheet. Its wording here is a summary of Annexure C for the review pack; the sheet filed with
// the PA is the PA's own form, signed. The chief executive and the chief accounting officer sign it (MR-04); they must be
// two different people. A paragraph about a form is included only when that form is in the set, and a deficiency is never
// left out: a form with a deficiency carries the qualification statement the regulation asks for.
type Declaration struct {
	Period                          string
	Forms                           []string
	Qualified                       map[string]string // form -> the statement of the deficiency or excess
	ChiefExecutive, ChiefAccounting string
	SignedAt                        time.Time
	Text                            string
}

// the paragraphs of DI 099 that depend on a form, in the form's order
var declarationParagraphs = []struct{ form, text string }{
	{"DI 100", "FORM DI 100: the issued primary and secondary share capital and primary and secondary unimpaired reserve funds have been reduced with the amounts of the items in section 48(5)(a)(i) to (iv) of the Act."},
	{"DI 310", "FORM DI 310: no liquid asset included in items 12 to 19 has been pledged or encumbered without the Registrar's consent (s.50(3)); securities have been valued under s.50(4); and the prescribed average daily minimum reserve balance and minimum liquid assets have been and will be maintained."},
	{"DI 400", "FORM DI 400: the prescribed minimum amount of issued primary and secondary share capital and unimpaired reserve funds (regulation 23(4)) has been and will be maintained."},
	{"DI 600", "FORM DI 600: the limits on the effective net open position in foreign currency (regulation 31(6)) have not been exceeded on any day of the reporting month."},
	{"DI 700", "FORM DI 700: sections 55 and 56 of the Act have been complied with at all times during the reporting period."},
}

// Declare builds the DI 099 for a set of computed returns and nil returns. It refuses a set containing a return with breaks, and
// refuses one person signing as both officers.
func Declare(period string, results []Result, nils []NilReturn, qualified map[string]string, ceo, cao string, at time.Time) (Declaration, error) {
	ceo, cao = strings.TrimSpace(ceo), strings.TrimSpace(cao)
	if ceo == "" || cao == "" || strings.EqualFold(ceo, cao) {
		return Declaration{}, fmt.Errorf("%w: the chief executive and the chief accounting officer must be two named people", ErrSameSigner)
	}
	d := Declaration{Period: period, Qualified: map[string]string{}, ChiefExecutive: ceo, ChiefAccounting: cao, SignedAt: at}
	in := map[string]bool{}
	for _, r := range results {
		if len(r.Breaks) > 0 {
			return Declaration{}, fmt.Errorf("%w: %s: %s", ErrBreak, r.Form, strings.Join(r.Breaks, "; "))
		}
		if r.Period != period {
			return Declaration{}, fmt.Errorf("%w: %s is for %s, not %s", ErrIncomplete, r.Form, r.Period, period)
		}
		in[r.Form] = true
	}
	for _, n := range nils {
		in[n.Form] = true
	}
	for f := range in {
		d.Forms = append(d.Forms, f)
	}
	sort.Strings(d.Forms)
	for f, why := range qualified {
		if !in[f] {
			return Declaration{}, fmt.Errorf("%w: a qualification for %s, which is not in this set", ErrIncomplete, f)
		}
		if strings.TrimSpace(why) == "" {
			return Declaration{}, fmt.Errorf("%w: the qualification for %s needs the statement of the deficiency", ErrIncomplete, f)
		}
		d.Qualified[f] = why
	}
	var b strings.Builder
	fmt.Fprintf(&b, "DI 099 DECLARATION IN RESPECT OF STATUTORY RETURNS SUBMITTED, period ended %s\n\n", period)
	b.WriteString("1. GENERAL: the information in the statements and returns is to the best of our knowledge and belief correct; they have been compiled in accordance with the requirements for mutual banks; and they reflect the management accounts as presented to the management and board.\n")
	n := 2
	for _, p := range declarationParagraphs {
		if !in[p.form] {
			continue
		}
		fmt.Fprintf(&b, "%d. %s", n, p.text)
		if q, ok := d.Qualified[p.form]; ok {
			fmt.Fprintf(&b, " QUALIFIED: %s", q)
		}
		b.WriteString("\n")
		n++
	}
	if len(nils) > 0 {
		b.WriteString("\nNil returns:\n")
		for _, x := range nils {
			fmt.Fprintf(&b, "- %s: %s", x.Form, x.Reason)
			if x.AttestedBy != "" {
				fmt.Fprintf(&b, " (confirmed by %s)", x.AttestedBy)
			}
			b.WriteString("\n")
		}
	}
	fmt.Fprintf(&b, "\nReturns in this set: %s\nChief executive officer: %s\nChief accounting officer: %s\n", strings.Join(d.Forms, ", "), ceo, cao)
	d.Text = b.String()
	return d, nil
}
