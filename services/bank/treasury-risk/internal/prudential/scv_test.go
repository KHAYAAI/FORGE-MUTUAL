package prudential

import "testing"

// Cover is per depositor across all their accounts, capped at the limit; foreign and unflagged deposits are uncovered.
func TestTheSingleCustomerViewCapsCoverPerDepositor(t *testing.T) {
	v, err := SingleCustomerView([]Deposit{
		{"a", "ZAR", true, 80_000_00}, {"a", "ZAR", true, 50_000_00}, // R130 000 eligible: R100 000 covered
		{"a", "USD", false, 1_000_00}, // $1 000 = R18 250 uncovered
		{"b", "ZAR", true, 20_000_00},
		{"c", "ZAR", false, 5_000_00}, // not flagged insured
		{"d", "ZAR", true, -10},       // an overdrawn balance is not a deposit
	}, CoverLimit, map[string]int64{"USD": 18_25000000})
	if err != nil {
		t.Fatal(err)
	}
	if len(v) != 3 {
		t.Fatalf("depositors = %+v", v)
	}
	a := v[0]
	if a.Accounts != 3 || a.Covered != 100_000_00 || a.Uncovered != 30_000_00+18_250_00 {
		t.Fatalf("a = %+v", a)
	}
	if v[1].Covered != 20_000_00 || v[1].Uncovered != 0 || v[2].Covered != 0 || v[2].Uncovered != 5_000_00 {
		t.Fatalf("b, c = %+v, %+v", v[1], v[2])
	}
}
