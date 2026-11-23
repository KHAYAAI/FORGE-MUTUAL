package interest

import "testing"

// R100,000.00 at 7.25% for a year of daily accruals is R7,250.00 to within a cent, and never more.
func TestAYearOfDailyAccrualMatchesTheAnnualRate(t *testing.T) {
	var accrued int64
	for day := 0; day < DaysInYear; day++ {
		d, err := Daily(10_000_000, 725)
		if err != nil {
			t.Fatal(err)
		}
		accrued += d
	}
	pay, carry := Payable(accrued, 0)
	if pay != 725_000 && pay != 724_999 {
		t.Fatalf("a year at 7.25%% on R100 000 paid %d cents", pay)
	}
	if pay*Nano+carry != accrued {
		t.Fatal("payment and carry do not add up to what accrued")
	}
}

// A small balance earns a fraction of a cent a day. Rounded to cents daily it would earn nothing; carried, it is paid.
func TestSmallBalancesEarnThroughTheCarry(t *testing.T) {
	d, _ := Daily(100, 500) // R1.00 at 5%: about 0.0137 cents a day
	if d == 0 {
		t.Fatal("a small balance earned nothing in a day")
	}
	var carry, paid int64
	for month := 0; month < 12; month++ {
		var accrued int64
		for day := 0; day < 30; day++ {
			accrued += d
		}
		p, c := Payable(accrued, carry)
		paid, carry = paid+p, c
	}
	if paid < 4 || paid > 5 { // 5% of R1.00 over 360 days is 4.9 cents
		t.Fatalf("a year on R1.00 at 5%% paid %d cents", paid)
	}
}

// Rounding never favours the customer: one day's interest times 365 is at most the annual rate.
func TestDailyInterestRoundsDown(t *testing.T) {
	for _, bal := range []int64{1, 7, 99, 12_345, 99_999_999, 1_000_000_000_000} {
		for _, rate := range []uint32{1, 375, 725, 1250, 10000} {
			d, err := Daily(bal, rate)
			if err != nil {
				t.Fatal(err)
			}
			if d*DaysInYear*10000 > bal*int64(rate)*Nano {
				t.Fatalf("balance %d at %d bps: daily %d overpays", bal, rate, d)
			}
		}
	}
}

func TestInvalidInputsAreRefused(t *testing.T) {
	if _, err := Daily(-1, 500); err == nil {
		t.Fatal("a negative balance accrued")
	}
	if _, err := Daily(100, 10001); err == nil {
		t.Fatal("a rate over 100% was accepted")
	}
	if d, _ := Daily(0, 500); d != 0 {
		t.Fatal("a zero balance earned interest")
	}
}
