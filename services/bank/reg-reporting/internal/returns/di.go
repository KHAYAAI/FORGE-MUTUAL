package returns

import (
	"embed"
	"encoding/csv"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// The mutual bank's DI 100 (balance sheet) and DI 200 (income statement) on the form's own line numbers.
//
// Source: the Prudential Authority's proposed directive for mutual banks on DI returns (13 August 2025), Annexures E1, E2, G1
// and G2, which carry the forms as published in the Government Gazette (DI 100: No. 22738, 5 October 2001; DI 200: No. 15381,
// 28 December 1993). See docs/regulatory/returns/SOURCES.md. The line numbers, labels and each total's arithmetic are in
// forms/di100.csv and forms/di200.csv, transcribed by reading the scans; a second person must check the transcription
// (CheckedBy stays empty until then, so no return is Submittable).
//
// Which ledger balance goes on which line is this repository's mapping, set out in mapDI100 and mapDI200 below. Where the
// mapping is a judgement it says so. The lines filled are the total column (DI 100 column 4 for liabilities and column 1 for
// assets; DI 200 the column the inputs cover). The maturity split, daily averages, yields, risk-weight columns and the cost
// of funds are not computed yet: UnbuiltColumns lists them on every result.

//go:embed forms/*.csv
var formFiles embed.FS

// FormLine is one line of a form as transcribed.
type FormLine struct {
	Section string
	No      int
	Label   string
	Sum     []Term // empty for a line that is reported directly
}

// Term is a signed range of lines in a total: "+5..8" or "-30".
type Term struct {
	Sign     Cents
	From, To int
}

// LoadForm reads a transcribed form: "di100" or "di200".
func LoadForm(name string) ([]FormLine, error) {
	f, err := formFiles.Open("forms/" + name + ".csv")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, err
	}
	var out []FormLine
	for i, r := range rows {
		if i == 0 {
			continue
		}
		if len(r) != 4 {
			return nil, fmt.Errorf("%s row %d: want 4 fields, got %d", name, i, len(r))
		}
		no, err := strconv.Atoi(r[1])
		if err != nil {
			return nil, fmt.Errorf("%s row %d: %w", name, i, err)
		}
		l := FormLine{Section: r[0], No: no, Label: r[2]}
		for _, tok := range strings.Fields(r[3]) {
			sign := Cents(1)
			switch tok[0] {
			case '+':
			case '-':
				sign = -1
			default:
				return nil, fmt.Errorf("%s line %d: term %q needs a sign", name, no, tok)
			}
			a, b, isRange := strings.Cut(tok[1:], "..")
			from, err := strconv.Atoi(a)
			if err != nil {
				return nil, fmt.Errorf("%s line %d: %w", name, no, err)
			}
			to := from
			if isRange {
				if to, err = strconv.Atoi(b); err != nil || to < from {
					return nil, fmt.Errorf("%s line %d: bad range %q", name, no, tok)
				}
			}
			l.Sum = append(l.Sum, Term{Sign: sign, From: from, To: to})
		}
		out = append(out, l)
	}
	return out, nil
}

// Position is one ledger balance at the end of the period: credits less debits in rand cents (a foreign-currency balance
// converted at the period-end rate). Product is the deposit or loan product id for customer accounts, the deposit product the
// interest was paid on for interest expense, or the system account key for system accounts; the mapping needs it where
// one ledger code covers several form lines.
type Position struct {
	Asset   string
	Code    uint32
	Product string
	Net     Cents
}

// ErrUnmapped means a position has no place on the form. It is a break, never silently dropped.
var ErrUnmapped = errors.New("returns: ledger position has no line on the form")

// UnbuiltColumns are the form columns this repository does not compute yet. Every DI result lists them.
var UnbuiltColumns = map[string][]string{
	"DI 100": {"liabilities columns 1 to 3 (short, medium and long term), 5 (daily average) and 6 (cost of funds)",
		"asset columns 2 (daily average), 3 (annualised gross yield), 4 to 10 (risk weightings) and 11 (impairments)"},
	"DI 200": {"the second column needs a second set of inputs: run the spec once with month and once with year-to-date positions"},
}

func isCurrent(product string) bool {
	return product == "current" || strings.HasSuffix(product, "-current")
}

// mapDI100 places a balance-sheet position. It returns the line and the value to add (assets shown as positives).
// Judgements: balances with the sponsor and partner banks are interbank placements (line 40) and a credit balance with
// them is interbank funding (line 3, or 10 in foreign currency); operating, FX pool and position, suspense, rewards,
// card-scheme settlement and distributions payable are "other" (line 22 when owed, line 74 when owing); income and
// expense codes accumulate as profit not yet appropriated (line 32), which must equal DI 200 line 66.
func mapDI100(p Position) (int, Cents, error) {
	zar := p.Asset == "ZAR"
	switch p.Code {
	case 1001:
		if !zar {
			return 11, p.Net, nil // foreign funding, non-banks
		}
		switch {
		case isCurrent(p.Product):
			return 5, p.Net, nil
		case p.Product == "savings":
			return 6, p.Net, nil
		case strings.HasPrefix(p.Product, "notice-"), strings.HasPrefix(p.Product, "fixed-"):
			return 7, p.Net, nil
		}
		return 0, 0, fmt.Errorf("%w: deposit product %q (code 1001) is not classed as demand, savings or notice", ErrUnmapped, p.Product)
	case 1010, 1020:
		if zar {
			return 47, -p.Net, nil
		}
		return 48, -p.Net, nil
	case 3001, 3002, 3003, 3010, 3020, 5001:
		if p.Net > 0 {
			if zar {
				return 3, p.Net, nil
			}
			return 10, p.Net, nil
		}
		return 40, -p.Net, nil
	case 2001, 2003, 2004, 2005, 3030, 5002, 8002:
		if p.Net >= 0 {
			return 22, p.Net, nil
		}
		return 74, -p.Net, nil
	case 2002, 2006, 2007, 2008:
		return 32, p.Net, nil
	case 8001:
		return 24, p.Net, nil
	}
	return 0, 0, fmt.Errorf("%w: account code %d", ErrUnmapped, p.Code)
}

// mapDI200 places an income or expense position (the period's movement). Balance-sheet codes are not income and are skipped.
// Judgements: fees from the FX service (system key "system:fx-fees:..." or product "fx") are foreign-exchange earnings
// (line 27); other fees are transaction-based (line 37); interest expense is split by the deposit product it was paid on.
func mapDI200(p Position) (int, Cents, bool, error) {
	switch p.Code {
	case 2008:
		if p.Asset == "ZAR" {
			return 8, p.Net, true, nil
		}
		return 9, p.Net, true, nil
	case 2007:
		switch {
		case p.Asset != "ZAR":
			return 18, -p.Net, true, nil
		case isCurrent(p.Product):
			return 14, -p.Net, true, nil
		case p.Product == "savings":
			return 15, -p.Net, true, nil
		case strings.HasPrefix(p.Product, "notice-"), strings.HasPrefix(p.Product, "fixed-"):
			return 16, -p.Net, true, nil
		}
		return 0, 0, true, fmt.Errorf("%w: interest expense must be split by deposit product, got %q", ErrUnmapped, p.Product)
	case 2002:
		if p.Product == "fx" || strings.HasPrefix(p.Product, "system:fx-fees") {
			return 27, p.Net, true, nil
		}
		return 37, p.Net, true, nil
	case 2006:
		return 42, -p.Net, true, nil
	}
	if _, _, err := mapDI100(p); err != nil {
		return 0, 0, false, err // unknown code: a break, as on the balance sheet
	}
	return 0, 0, false, nil
}

func leafName(form string, no int) string {
	return fmt.Sprintf("%s.%d", strings.ReplaceAll(form, " ", ""), no)
}

// DIInputs turns positions into the leaf-line inputs of DI 100 and DI 200. openingRetained is DI 200 line 65 (retained income
// at the beginning of the period, from the last audited year end). Any position that cannot be placed is returned as a
// problem, and the caller must not file a return with problems.
func DIInputs(ps []Position, openingRetained Cents) (map[string]Cents, []string) {
	in := map[string]Cents{}
	for _, form := range []struct {
		name, file string
	}{{"DI 100", "di100"}, {"DI 200", "di200"}} {
		lines, err := LoadForm(form.file)
		if err != nil {
			return nil, []string{err.Error()}
		}
		for _, l := range lines {
			if len(l.Sum) == 0 {
				in[leafName(form.name, l.No)] = 0
			}
		}
	}
	var problems []string
	for _, p := range ps {
		line, v, err := mapDI100(p)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s %d %q: %v", p.Asset, p.Code, p.Product, err))
			continue
		}
		in[leafName("DI 100", line)] += v
		line, v, income, err := mapDI200(p)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s %d %q: %v", p.Asset, p.Code, p.Product, err))
			continue
		}
		if income {
			in[leafName("DI 200", line)] += v
		}
	}
	in[leafName("DI 200", 65)] += openingRetained
	sort.Strings(problems)
	return in, problems
}

// diSpec builds a Spec from a transcribed form: leaf lines read their input, totals follow the form's own arithmetic.
func diSpec(form, file, name, frequency string) (Spec, error) {
	lines, err := LoadForm(file)
	if err != nil {
		return Spec{}, err
	}
	byNo := map[int]FormLine{}
	for _, l := range lines {
		byNo[l.No] = l
	}
	var eval func(no int, in map[string]Cents, depth int) (Cents, error)
	eval = func(no int, in map[string]Cents, depth int) (Cents, error) {
		l, ok := byNo[no]
		if !ok || depth > len(lines) {
			return 0, fmt.Errorf("%s: total refers to line %d, which is not on the form or loops", form, no)
		}
		if len(l.Sum) == 0 {
			return in[leafName(form, no)], nil
		}
		var s Cents
		for _, t := range l.Sum {
			for n := t.From; n <= t.To; n++ {
				v, err := eval(n, in, depth+1)
				if err != nil {
					return 0, err
				}
				s += t.Sign * v
			}
		}
		return s, nil
	}
	s := Spec{Form: form, Name: name, Frequency: frequency, Audited: true}
	for _, l := range lines {
		l := l
		ref := leafName(form, l.No)
		line := Line{Ref: ref, Label: l.Label}
		if len(l.Sum) == 0 {
			line.Inputs = []string{ref}
		}
		line.Compute = func(in map[string]Cents) (Cents, error) { return eval(l.No, in, 0) }
		s.Lines = append(s.Lines, line)
	}
	return s, nil
}

// DI100 is the mutual bank's balance sheet. Total assets (line 76) must equal total liabilities (line 34).
func DI100() (Spec, error) {
	s, err := diSpec("DI 100", "di100", "Balance sheet", "monthly")
	if err != nil {
		return s, err
	}
	s.Reconcile = map[string]func(map[string]Cents) (Cents, error){
		"DI100.76": func(in map[string]Cents) (Cents, error) {
			for _, l := range s.Lines {
				if l.Ref == "DI100.34" {
					return l.Compute(in)
				}
			}
			return 0, ErrIncomplete
		},
	}
	return s, nil
}

// DI200 is the mutual bank's income statement.
func DI200() (Spec, error) { return diSpec("DI 200", "di200", "Income statement", "monthly") }

// CrossCheck compares the two forms: profit not appropriated on DI 100 (line 32) is DI 200's retained income at the end of the
// period (line 66). It returns a break description, or "" when they agree.
func CrossCheck(di100, di200 Result) string {
	a, b := di100.Values["DI100.32"], di200.Values["DI200.66"]
	if a != b {
		return fmt.Sprintf("DI 100 line 32 is %d but DI 200 line 66 is %d", a, b)
	}
	return ""
}

// Thousands rounds cents to the form's unit, R'000, half away from zero.
func Thousands(c Cents) int64 {
	const unit = 100_000
	if c < 0 {
		return -int64((-c + unit/2) / unit)
	}
	return int64((c + unit/2) / unit)
}

// FormTable renders a DI result in the form's unit (R'000), every line in order, with the lines the form totals shown in bold.
func FormTable(s Spec, r Result) string {
	var b strings.Builder
	fmt.Fprintf(&b, "| %s %s, %s | Line | R'000 |\n|---|---:|---:|\n", s.Form, s.Name, r.Period)
	file := strings.ToLower(strings.ReplaceAll(s.Form, " ", ""))
	lines, _ := LoadForm(file)
	total := map[int]bool{}
	for _, l := range lines {
		total[l.No] = len(l.Sum) > 0
	}
	for _, l := range s.Lines {
		var no int
		fmt.Sscanf(l.Ref[strings.Index(l.Ref, ".")+1:], "%d", &no)
		label := l.Label
		if total[no] {
			label = "**" + label + "**"
		}
		fmt.Fprintf(&b, "| %s | %d | %d |\n", strings.ReplaceAll(label, "|", "/"), no, Thousands(r.Values[l.Ref]))
	}
	if cols := UnbuiltColumns[s.Form]; len(cols) > 0 {
		fmt.Fprintf(&b, "\nNot computed: %s.\n", strings.Join(cols, "; "))
	}
	return b.String()
}
