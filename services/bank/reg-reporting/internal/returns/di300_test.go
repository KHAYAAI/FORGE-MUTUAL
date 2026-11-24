package returns

import (
	"errors"
	"testing"
	"time"
)

func TestDI300LaddersDepositsByNoticeAndLoansBySchedule(t *testing.T) {
	ps := book()
	r100, _ := computeDI(t, ps, 0)
	loans := r100.Values["DI100.39"] // includes balances with banks (line 40), which are 0-31 days
	in := DI300Inputs{DI100: r100, Positions: ps, LoanMaturities: [6]Cents{loans - 320_000_00, 20_000_00, 0, 100_000_00, 50_000_00, 150_000_00}, TenLargestDepositors: 700_000_00}
	r, err := DI300("2027-12", in, time.Now())
	if err != nil || len(r.Breaks) != 0 {
		t.Fatalf("%v %v", r.Breaks, err)
	}
	want := map[string]Cents{
		"DI300.8.1": 1_000_000_00, "DI300.8.2": 850_000_00, "DI300.8.3": 100_000_00, "DI300.8.4": 50_000_00, // notice-32 and notice-90
		"DI300.9.2": 180_000_00, "DI300.14.1": 0, "DI300.31.1": 1_000_000_00, "DI300.33.1": 700_000_00,
	}
	for ref, v := range want {
		if r.Values[ref] != v {
			t.Fatalf("%s = %d, want %d", ref, r.Values[ref], v)
		}
	}
	// The cumulative mismatch ends at nil, because the balance sheet balances.
	if r.Values["DI300.18.7"] != 0 {
		t.Fatalf("cumulative mismatch ends at %d", r.Values["DI300.18.7"])
	}
	in.LoanMaturities[0]--
	if _, err := DI300("2027-12", in, time.Now()); !errors.Is(err, ErrIncomplete) {
		t.Fatal("a loan schedule that does not match DI 100 must be refused")
	}
}

func TestNoticeColumns(t *testing.T) {
	for product, col := range map[string]int{"business-zar-current": 2, "savings": 2, "notice-32": 3, "notice-60": 3, "notice-90": 4, "notice-180": 5, "notice-365": 6, "notice-400": 7} {
		if got := noticeColumn(product); got != col {
			t.Fatalf("%s: column %d, want %d", product, got, col)
		}
	}
}
