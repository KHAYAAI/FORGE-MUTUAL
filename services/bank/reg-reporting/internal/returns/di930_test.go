package returns

import (
	"errors"
	"testing"
	"time"
)

func TestDI930ReportsThePredominantRatePerCategory(t *testing.T) {
	d := func(day int) time.Time { return time.Date(2027, 11, day, 0, 0, 0, 0, time.UTC) }
	accts := []RateAccount{
		{Product: "business-zar-current", RateBps: 0, EffectiveFrom: d(1), Balance: 10},
		{Product: "business-zar-current", RateBps: 0, EffectiveFrom: d(1), Balance: 10},
		{Product: "business-zar-current", RateBps: 100, EffectiveFrom: d(5), Balance: 1_000_000_00},
		{Product: "savings", RateBps: 650, EffectiveFrom: d(3), Balance: 50},
		{Product: "savings", RateBps: 600, EffectiveFrom: d(2), Balance: 500}, // tie on count: the larger balance wins
		{Product: "notice-32", RateBps: 775, EffectiveFrom: d(10), Balance: 200_000_00, OpenedInMonth: true},
		{Product: "notice-32", RateBps: 775, EffectiveFrom: d(10), Balance: 50_000_00, OpenedInMonth: true}, // under R100 000: not a "new deposit" line
		{Product: "notice-90", RateBps: 850, EffectiveFrom: d(12), Balance: 150_000_00, OpenedInMonth: true},
		{Product: "working-capital", RateBps: 1450, EffectiveFrom: d(1), Balance: -5, OverdraftLimit: true},
		{Product: "rbf", RateBps: 1600},
	}
	r, err := DI930("2027-11", accts, 1125, d(1), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]Cents{
		"DI930.1.2": 0, "DI930.1.1": 20271101, "DI930.3.2": 600, "DI930.3.1": 20271102, "DI930.12.2": 775, "DI930.13.2": 850,
		"DI930.17.2": 1125, "DI930.18.2": 1450, "DI930.21.2": 0, "DI930.21.1": 0,
	}
	for ref, v := range want {
		if r.Values[ref] != v {
			t.Fatalf("%s = %d, want %d", ref, r.Values[ref], v)
		}
	}
	if len(DI930Lines) != 29 {
		t.Fatal("29 lines")
	}
	if _, err := DI930("p", accts, 0, d(1), time.Now()); !errors.Is(err, ErrIncomplete) {
		t.Fatal("prime is required")
	}
}
