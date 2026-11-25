package returns

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// DI 930, interest rates on deposits, loans and advances at month-end (monthly, a statistical return), lines 1 to 29. Source: the PA's
// proposed directive for mutual banks (13 August 2025), Annexure ZZ (Government Gazette No. 15381, 28 December 1993). Transcribed by
// reading the scan; a second person must check it.
//
// Each line has two columns: 1 the date from which the rate is effective, stored here as yyyymmdd, and 2 the rate, stored in basis
// points. A "predominant rate" is the rate on the largest number of accounts in the category (the form's note); a tie goes to the
// rate on the larger total balance. A line with no accounts is left empty (both columns zero). Cell references "DI930.<line>.<column>".
//
// Forge's products fill: 1 cheque (current) accounts; 3 savings; 12 and 13 new 32-day and 88-to-91-day notice deposits over R100 000;
// 10 new call deposits over R100 000; 17 prime; 18 overdrafts on current accounts. Transmission accounts, state-assisted housing
// savings, fixed deposits, the interbank call rate, instalment sales and mortgages are not offered.

// DI930Lines are the form's labels, by line.
var DI930Lines = map[int]string{
	1: "Cheque accounts - predominant rate on credit balances", 2: "Transmission accounts - predominant rate", 3: "Savings accounts - predominant rate",
	4: "State assisted home ownership savings scheme", 5: "Fixed deposits with original maturity of 12 to _ months", 6: "Fixed deposits 18 to _ months",
	7: "Fixed deposits 24 to _ months", 8: "Fixed deposits 36 to _ months", 9: "Fixed deposits 48 to _ months",
	10: "New deposits of more than R100 000 by the public: call deposits", 11: "New deposits over R100 000: 7 days notice",
	12: "New deposits over R100 000: 32 days notice", 13: "New deposits over R100 000: 88 to 91 days notice",
	14: "New deposits over R100 000: 182 to 185 days notice", 15: "New deposits over R100 000: 12 months fixed", 16: "Predominant inter-bank call rate",
	17: "Lending rates: prime overdraft rate", 18: "Predominant overdraft rate on current accounts",
	19: "Predominant rate on new fixed rate instalment sale agreements", 20: "Predominant rate on adjustable rate instalment sale agreements",
	21: "Adjustable rate mortgage advances: dwelling houses and sectional title units", 22: "Adjustable rate mortgage advances: flats",
	23: "Adjustable rate mortgage advances: other structures", 24: "Adjustable rate mortgage advances: vacant urban land",
	25: "Adjustable rate mortgage advances: farm property", 26: "New fixed rate mortgage advances: period and rate (1)",
	27: "New fixed rate mortgage advances: period and rate (2)", 28: "New fixed rate mortgage advances: period and rate (3)",
	29: "New fixed rate mortgage advances: period and rate (4)",
}

// RateAccount is one account at month-end.
type RateAccount struct {
	Product        string
	RateBps        int64
	EffectiveFrom  time.Time
	Balance        Cents
	OpenedInMonth  bool // a new deposit placed during the month
	OverdraftLimit bool // a current account with an overdraft facility in use (lending)
}

// DI930Line places an account on a line, or 0 if no line takes it.
func DI930Line(a RateAccount) int {
	switch {
	case a.OverdraftLimit:
		return 18
	case a.OpenedInMonth && a.Balance > 100_000_00 && strings.HasPrefix(a.Product, "notice-32"):
		return 12
	case a.OpenedInMonth && a.Balance > 100_000_00 && (strings.HasPrefix(a.Product, "notice-88") || strings.HasPrefix(a.Product, "notice-90") || strings.HasPrefix(a.Product, "notice-91")):
		return 13
	case a.OpenedInMonth && a.Balance > 100_000_00 && (isCurrent(a.Product) || a.Product == "savings"):
		return 10
	case isCurrent(a.Product):
		return 1
	case a.Product == "savings":
		return 3
	}
	return 0
}

// DI930 computes the return. Prime (line 17) is the bank's published prime rate.
func DI930(period string, accounts []RateAccount, primeBps int64, primeFrom time.Time, at time.Time) (Result, error) {
	if primeBps <= 0 || primeFrom.IsZero() {
		return Result{}, fmt.Errorf("%w: DI 930 needs the prime rate and the date it took effect", ErrIncomplete)
	}
	type tally struct {
		n       int
		balance Cents
		from    time.Time
	}
	byLine := map[int]map[int64]*tally{}
	for _, a := range accounts {
		line := DI930Line(a)
		if line == 0 {
			continue
		}
		if byLine[line] == nil {
			byLine[line] = map[int64]*tally{}
		}
		t := byLine[line][a.RateBps]
		if t == nil {
			t = &tally{}
			byLine[line][a.RateBps] = t
		}
		t.n++
		t.balance += a.Balance
		if t.from.IsZero() || a.EffectiveFrom.Before(t.from) {
			t.from = a.EffectiveFrom
		}
	}
	r := Result{Form: "DI 930", Period: period, Values: map[string]Cents{}, ComputedAt: at}
	ymd := func(t time.Time) Cents { return Cents(t.Year()*10000 + int(t.Month())*100 + t.Day()) }
	for line := 1; line <= 29; line++ {
		r.Values[fmt.Sprintf("DI930.%d.1", line)] = 0
		r.Values[fmt.Sprintf("DI930.%d.2", line)] = 0
		rates := byLine[line]
		if len(rates) == 0 {
			continue
		}
		var best int64
		var bt *tally
		for rate, t := range rates {
			if bt == nil || t.n > bt.n || (t.n == bt.n && (t.balance > bt.balance || (t.balance == bt.balance && rate < best))) {
				best, bt = rate, t
			}
		}
		r.Values[fmt.Sprintf("DI930.%d.1", line)] = ymd(bt.from)
		r.Values[fmt.Sprintf("DI930.%d.2", line)] = Cents(best)
	}
	r.Values["DI930.17.1"] = ymd(primeFrom)
	r.Values["DI930.17.2"] = Cents(primeBps)
	for ref := range r.Values {
		r.Unverified = append(r.Unverified, ref)
	}
	sort.Strings(r.Unverified)
	return r, nil
}
