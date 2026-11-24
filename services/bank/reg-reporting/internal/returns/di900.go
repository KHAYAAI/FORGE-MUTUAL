package returns

import (
	"fmt"
	"strings"
)

// DI 900, institutional and maturity breakdown of liabilities and assets (monthly statistical return, a public document), lines 1 to
// 230 (there is no line 81). Source: the PA's proposed directive for mutual banks (13 August 2025), Annexures X1 (8 pages, Government
// Gazette No. 15381, 28 December 1993) and X2 (which maps DI 100 and DI 110 lines to DI 900 lines). Transcribed by reading the scans; a
// second person must check it. Totals follow the form; lines 80 and 223 (total capital and liabilities, total assets) must agree.
//
// Only the total of each line is computed; the deposit maturity columns, the "of which financial rand" column and the domestic/foreign
// split of assets are not (UnbuiltColumns). Institutional sectors follow the SARB's Institutional Sector Classification Guide.
// Judgements: a deposit or loan product whose id starts with "personal-" belongs to an individual, anything else to a company; foreign-
// currency deposits are from domestic parties (line 32); balances with partner banks in foreign currency are deposits with SA banks
// (line 153; VERIFY where a partner bank is not South African, line 155); retained profit is primary reserve funds (line 77); member
// shares are held by companies (line 74).

func init() {
	UnbuiltColumns["DI 900"] = []string{"deposit columns 1 to 7 and 9 (by type and maturity, financial rand) and the domestic/foreign split of assets; only line totals are computed"}
}

func personal(product string) bool { return strings.HasPrefix(product, "personal-") }

// mapDI900 places a position on a DI 900 leaf line, through its DI 100 line.
func mapDI900(p Position) (int, Cents, error) {
	line, v, err := mapDI100(p)
	if err != nil {
		return 0, 0, err
	}
	zar := p.Asset == "ZAR"
	switch line {
	case 5, 6, 7:
		if personal(p.Product) {
			return 21, v, nil
		}
		return 19, v, nil
	case 11:
		return 32, v, nil
	case 3:
		return 8, v, nil
	case 10:
		return 29, v, nil
	case 22:
		return 67, v, nil
	case 24:
		return 74, v, nil
	case 32:
		return 77, v, nil
	case 40:
		if zar {
			return 118, v, nil
		}
		return 153, v, nil
	case 47:
		od := strings.Contains(p.Product, "overdraft")
		switch {
		case od && personal(p.Product):
			return 176, v, nil
		case od:
			return 174, v, nil
		case personal(p.Product):
			return 181, v, nil
		}
		return 179, v, nil
	case 48:
		return 154, v, nil
	case 74:
		return 222, v, nil
	}
	return 0, 0, fmt.Errorf("%w: DI 100 line %d has no DI 900 line yet", ErrUnmapped, line)
}

// DI900Inputs places positions on DI 900 leaf lines. netOpenFX is DI 110 line 6 (DI 900 line 86); overdraftLimits the total of
// overdraft limits loaded (line 88).
func DI900Inputs(ps []Position, netOpenFX, overdraftLimits Cents) (map[string]Cents, []string) {
	lines, err := LoadForm("di900")
	if err != nil {
		return nil, []string{err.Error()}
	}
	in := map[string]Cents{}
	for _, l := range lines {
		if len(l.Sum) == 0 {
			in[leafName("DI 900", l.No)] = 0
		}
	}
	var problems []string
	for _, p := range ps {
		line, v, err := mapDI900(p)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s %d %q: %v", p.Asset, p.Code, p.Product, err))
			continue
		}
		in[leafName("DI 900", line)] += v
	}
	in["DI900.86"] = netOpenFX
	in["DI900.88"] = overdraftLimits
	return in, problems
}

// DI900 is the return. Total assets (line 223) must equal total capital and liabilities (line 80).
func DI900() (Spec, error) {
	s, err := diSpec("DI 900", "di900", "Institutional and maturity breakdown of liabilities and assets", "monthly")
	if err != nil {
		return s, err
	}
	s.Audited = false // a statistical return to the Reserve Bank
	s.Reconcile = map[string]func(map[string]Cents) (Cents, error){
		"DI900.223": func(in map[string]Cents) (Cents, error) {
			for _, l := range s.Lines {
				if l.Ref == "DI900.80" {
					return l.Compute(in)
				}
			}
			return 0, ErrIncomplete
		},
	}
	return s, nil
}
