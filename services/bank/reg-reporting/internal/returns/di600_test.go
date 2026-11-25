package returns

import (
	"errors"
	"testing"
	"time"
)

func TestDI600ConvertsToDollarsAndMeasuresTheOpenPosition(t *testing.T) {
	r100, _ := computeDI(t, book(), 0)
	ps := []Position{
		{"USD", 1001, "usd-current", 182_500_00}, // $10 000 owed to customers
		{"USD", 5001, "", -200_750_00},           // $11 000 at the partner bank: long $1 000
		{"EUR", 1001, "eur-current", 36_500_00},  // $2 000 owed, nothing held: short $2 000
		{"GBP", 5001, "", -18_250_00},            // $1 000 held: long $1 000
	}
	r, err := DI600("2027-12", DI600Inputs{Positions: ps, RandCentsPerUSD: 1825, DI100: r100}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]Cents{
		"DI600.1.1": 11_000_00, "DI600.2.1": 10_000_00, "DI600.6.1": 1_000_00, "DI600.6.7": -2_000_00, "DI600.6.2": 1_000_00,
		"DI600.8.8":  2_000_00, // longs 2 000, shorts 2 000: the larger is 2 000
		"DI600.13.1": 11_000_00, "DI600.14.1": 10_000_00, "DI600.18.9": 1_000_00, "DI600.24.9": -1_000_00,
	}
	for ref, v := range want {
		if r.Values[ref] != v {
			t.Fatalf("%s = %d, want %d", ref, r.Values[ref], v)
		}
	}
	// The limit is 10% of DI 100 line 31 in dollars: R1 000 of capital is $54.79, so a $2 000 position is far over it.
	if r.Values["DI600.9.8"] != r100.Values["DI100.31"]*100/1825/10 {
		t.Fatalf("limit %d", r.Values["DI600.9.8"])
	}
	if len(Shortfalls(r)) != 1 {
		t.Fatalf("an open position over the limit must be reported: %v", Shortfalls(r))
	}
	if NetOpenRand(r, 1825) != 36_500_00 {
		t.Fatalf("back to rand: %d", NetOpenRand(r, 1825))
	}
	if _, err := DI600("p", DI600Inputs{Positions: ps, DI100: r100}, time.Now()); !errors.Is(err, ErrIncomplete) {
		t.Fatal("the rate is required")
	}
}
