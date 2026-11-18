package credit

import "testing"

// R100 000 over 12 months at 12% is about R8 884.88 a month, R106 618.55 in all; the schedule repays exactly the
// principal, and each month's interest is on the balance left.
func TestATermLoanRepaysExactlyThePrincipal(t *testing.T) {
	s, err := Schedule(10_000_000, 1200, 12)
	if err != nil {
		t.Fatal(err)
	}
	var principal int64
	for _, i := range s {
		principal += i.Principal
	}
	if principal != 10_000_000 || len(s) != 12 {
		t.Fatalf("principal repaid %d over %d instalments", principal, len(s))
	}
	if s[0].Interest != 100_000 || s[0].Total() != 888_488 {
		t.Fatalf("first instalment = %+v (total %d), want interest 100000 and total 888488", s[0], s[0].Total())
	}
	total := TotalRepayable(s)
	if total < 10_661_800 || total > 10_661_900 {
		t.Fatalf("total repayable = %d", total)
	}
	for k := 1; k < len(s); k++ {
		if s[k].Interest > s[k-1].Interest {
			t.Fatal("interest grew while the balance fell")
		}
	}
}

func TestAnInterestFreeLoanSplitsEvenly(t *testing.T) {
	s, _ := Schedule(1000, 0, 3)
	if TotalRepayable(s) != 1000 || s[0].Principal != 334 || s[2].Principal != 332 {
		t.Fatalf("schedule = %+v", s)
	}
}

func TestInvalidLoansAreRefused(t *testing.T) {
	for _, c := range []struct {
		p int64
		r uint32
		m int
	}{{0, 1000, 12}, {100, 10001, 12}, {100, 1000, 0}, {100, 1000, 361}} {
		if _, err := Schedule(c.p, c.r, c.m); err == nil {
			t.Fatalf("%+v accepted", c)
		}
	}
}

// A revenue-based sweep takes its share, splits it between principal and fee in proportion, and never takes more
// than is owed.
func TestTheSweepTakesItsShareAndNoMore(t *testing.T) {
	p, f := Sweep(1_000_000, 1000, 900_000, 100_000) // 10% of R10 000 = R1 000; owed 90% principal, 10% fee
	if p+f != 100_000 || f != 10_000 {
		t.Fatalf("sweep = %d principal, %d fee", p, f)
	}
	p, f = Sweep(10_000_000, 1000, 50_000, 5_000) // the share is more than is owed: take only what is owed
	if p != 50_000 || f != 5_000 {
		t.Fatalf("final sweep = %d, %d", p, f)
	}
	if p, f := Sweep(1000, 1000, 0, 0); p != 0 || f != 0 {
		t.Fatal("a settled advance was swept")
	}
	if Fee(10_000_000, 800) != 800_000 {
		t.Fatal("an 8% fee on R100 000 is R8 000")
	}
}

func TestOverdraftInterestAndAffordability(t *testing.T) {
	if DailyNano(-5, 1500) != 0 || DailyNano(0, 1500) != 0 {
		t.Fatal("no balance drawn, no interest")
	}
	if SuggestedMax("term-loan", 9_000_000) != 9_000_000 || SuggestedMax("overdraft", 9_000_000) != 3_000_000 {
		t.Fatal("affordability multiples changed")
	}
}
