package returns

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// A summary balance sheet and income statement, built from the ledger's trial balance, for either legal form: the bank's
// BA 100 and BA 120 (form numbers confirmed in Directive 1 of 2025, Annexure 1A), still on this repository's own line
// references. The mutual bank's filed forms, DI 100 and DI 200, are built on the form's own line numbers in di.go; the
// summary here stays for the bank route and for management packs.
//
// Every line reads the net balance (credits less debits, in rand cents) of named ledger account codes, given as inputs
// "code:<n>". The balance sheet must balance: total assets equal total liabilities and equity, which holds only when
// the ledger is balanced and every code is mapped. An unmapped code is a break, never silently dropped.
//
// Line references are this repository's own ("BS.1" and so on) until someone maps them to the form's line numbers
// from the PA's form instructions; until then every line is unverified and the return cannot be submitted.

// LegalForm names which set of forms to build: "mutual" or "bank".
type LegalForm string

const (
	Mutual LegalForm = "mutual"
	Bank   LegalForm = "bank"
)

// FormNumbers are the form numbers for each statement by legal form.
var FormNumbers = map[LegalForm]map[string]string{
	Bank:   {"balance-sheet": "BA 100", "income-statement": "BA 120"},
	Mutual: {"balance-sheet": "DI 100", "income-statement": "DI 200"},
}

// Classes say where each ledger account code goes. Debit-normal codes are negated so the statement shows positives.
var (
	cashCodes        = []uint32{3001, 3002, 3003, 3010, 3020, 5001}
	loanCodes        = []uint32{1010, 1020}
	depositCodes     = []uint32{1001}
	otherLiabCodes   = []uint32{2001, 2003, 2004, 2005, 3030, 5002, 8002}
	incomeCodes      = []uint32{2002, 2008}
	expenseCodes     = []uint32{2006, 2007}
	capitalCodes     = []uint32{8001} // member share capital (mutual) or share capital paid in
	statementMapping = func() map[uint32]bool {
		m := map[uint32]bool{}
		for _, cs := range [][]uint32{cashCodes, loanCodes, depositCodes, otherLiabCodes, incomeCodes, expenseCodes, capitalCodes} {
			for _, c := range cs {
				m[c] = true
			}
		}
		return m
	}()
)

func names(codes []uint32) []string {
	out := make([]string, len(codes))
	for i, c := range codes {
		out[i] = "code:" + strconv.Itoa(int(c))
	}
	return out
}

func sum(in map[string]Cents, codes []uint32, sign Cents) Cents {
	var s Cents
	for _, n := range names(codes) {
		s += in[n]
	}
	return s * sign
}

func line(ref, label string, codes []uint32, sign Cents) Line {
	return Line{Ref: ref, Label: label, Inputs: names(codes), Compute: func(in map[string]Cents) (Cents, error) { return sum(in, codes, sign), nil }}
}

// Inputs turns trial balance nets (code to credits less debits, already in rand) into return inputs. It reports any
// code the statements do not map, so a new account code cannot vanish from the returns.
func Inputs(nets map[uint32]int64) (map[string]Cents, []uint32) {
	in := map[string]Cents{}
	for c := range statementMapping {
		in["code:"+strconv.Itoa(int(c))] = 0
	}
	var unmapped []uint32
	for c, v := range nets {
		if !statementMapping[c] {
			unmapped = append(unmapped, c)
			continue
		}
		in["code:"+strconv.Itoa(int(c))] = Cents(v)
	}
	sort.Slice(unmapped, func(i, j int) bool { return unmapped[i] < unmapped[j] })
	return in, unmapped
}

// BalanceSheet is the balance sheet return for a legal form. Current-period profit sits in equity as retained
// earnings, so the sheet balances before the year is closed.
func BalanceSheet(form LegalForm) (Spec, error) {
	num, ok := FormNumbers[form]["balance-sheet"]
	if !ok {
		return Spec{}, fmt.Errorf("returns: unknown legal form %q", form)
	}
	equity := "Equity: share capital and retained earnings (current period)"
	if form == Mutual {
		equity = "Members' shares and reserves: share capital and retained surplus (current period)"
	}
	s := Spec{Form: num, Name: "Balance sheet", Frequency: "monthly", Audited: true,
		Lines: []Line{
			line("BS.1", "Cash and balances with the sponsor, partner banks and settlement accounts", cashCodes, -1),
			line("BS.2", "Loans and advances to customers", loanCodes, -1),
			{Ref: "BS.3", Label: "Total assets", Inputs: names(append(append([]uint32{}, cashCodes...), loanCodes...)),
				Compute: func(in map[string]Cents) (Cents, error) { return sum(in, cashCodes, -1) + sum(in, loanCodes, -1), nil }},
			line("BS.4", "Deposits from customers", depositCodes, 1),
			line("BS.5", "Other liabilities: operating, FX pool and position, suspense, rewards", otherLiabCodes, 1),
			{Ref: "BS.6", Label: equity, Inputs: names(append(append(append([]uint32{}, incomeCodes...), expenseCodes...), capitalCodes...)),
				Compute: func(in map[string]Cents) (Cents, error) {
					return sum(in, incomeCodes, 1) + sum(in, expenseCodes, 1) + sum(in, capitalCodes, 1), nil
				}},
			{Ref: "BS.7", Label: "Total liabilities and equity", Inputs: nil,
				Compute: func(in map[string]Cents) (Cents, error) {
					return sum(in, depositCodes, 1) + sum(in, otherLiabCodes, 1) + sum(in, incomeCodes, 1) + sum(in, expenseCodes, 1) + sum(in, capitalCodes, 1), nil
				}},
		},
		Totals: []Total{{Line: "BS.3", Sum: []string{"BS.1", "BS.2"}}, {Line: "BS.7", Sum: []string{"BS.4", "BS.5", "BS.6"}}},
		// The sheet balances: assets come from the debit side, liabilities and equity from the credit side.
		Reconcile: map[string]func(map[string]Cents) (Cents, error){
			"BS.3": func(in map[string]Cents) (Cents, error) {
				return sum(in, depositCodes, 1) + sum(in, otherLiabCodes, 1) + sum(in, incomeCodes, 1) + sum(in, expenseCodes, 1) + sum(in, capitalCodes, 1), nil
			},
		},
	}
	return s, nil
}

// IncomeStatement is the income statement return for a legal form.
func IncomeStatement(form LegalForm) (Spec, error) {
	num, ok := FormNumbers[form]["income-statement"]
	if !ok {
		return Spec{}, fmt.Errorf("returns: unknown legal form %q", form)
	}
	result := "Profit for the period"
	if form == Mutual {
		result = "Surplus for the period (available to members after reserves)"
	}
	return Spec{Form: num, Name: "Income statement", Frequency: "monthly", Audited: true,
		Lines: []Line{
			line("IS.1", "Interest income", []uint32{2008}, 1),
			line("IS.2", "Interest expense", []uint32{2007}, -1),
			{Ref: "IS.3", Label: "Net interest income", Inputs: names([]uint32{2007, 2008}),
				Compute: func(in map[string]Cents) (Cents, error) {
					return sum(in, []uint32{2008}, 1) + sum(in, []uint32{2007}, 1), nil
				}},
			line("IS.4", "Fee and commission income", []uint32{2002}, 1),
			line("IS.5", "Credit losses written off", []uint32{2006}, -1),
			{Ref: "IS.6", Label: result, Inputs: names(append(append([]uint32{}, incomeCodes...), expenseCodes...)),
				Compute: func(in map[string]Cents) (Cents, error) {
					return sum(in, incomeCodes, 1) + sum(in, expenseCodes, 1), nil
				}},
		},
		// IS.6 = IS.3 + IS.4 - IS.5, checked by reconciliation because IS.5 is shown as a positive loss.
		Reconcile: map[string]func(map[string]Cents) (Cents, error){
			"IS.6": func(in map[string]Cents) (Cents, error) {
				return sum(in, []uint32{2008}, 1) + sum(in, []uint32{2007}, 1) + sum(in, []uint32{2002}, 1) + sum(in, []uint32{2006}, 1), nil
			},
		},
	}, nil
}

// Table renders a computed return as Markdown, for review packs and pro formas.
func Table(s Spec, r Result) string {
	var b strings.Builder
	fmt.Fprintf(&b, "| %s %s, %s | Rand |\n|---|---:|\n", s.Form, s.Name, r.Period)
	for _, l := range s.Lines {
		fmt.Fprintf(&b, "| %s %s | %s |\n", l.Ref, l.Label, Rand(r.Values[l.Ref]))
	}
	return b.String()
}

// Rand formats cents as rand with thousands separators: 1234567 -> "12 345.67".
func Rand(c Cents) string {
	neg := c < 0
	if neg {
		c = -c
	}
	whole, cents := int64(c)/100, int64(c)%100
	s := strconv.FormatInt(whole, 10)
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	parts = append([]string{s}, parts...)
	out := fmt.Sprintf("%s.%02d", strings.Join(parts, " "), cents)
	if neg {
		out = "-" + out
	}
	return out
}
