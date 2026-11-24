package returns

import (
	"errors"
	"testing"
	"time"
)

func TestDI510ListsExposuresAboveTenPercentOfCapital(t *testing.T) {
	r100, _ := computeDI(t, book(), 0) // net qualifying capital (line 31) R10 000
	ex := []Exposure{
		{Person: "Sponsor Bank Ltd", Nature: "deposit placed with bank", Granted: 1_500_000_00, Utilised: 1_500_000_00},
		{Person: "Acme (Pty) Ltd", Nature: "working-capital loan", Granted: 2_000_00, Utilised: 1_500_00, Overdue: 500_00, SecurityOverdue: 100_00, SpecificProvisions: 200_00},
		{Person: "Small Co", Nature: "overdraft", Granted: 1_000_00, Utilised: 1_000_00}, // exactly 10%: not above it
	}
	r, err := DI510("2027-Q4", ex, r100, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Rows) != 2 || r.Rows[0].Person != "Sponsor Bank Ltd" {
		t.Fatalf("rows %+v", r.Rows)
	}
	want := map[string]Cents{"DI510.1.4": 1_502_000_00, "DI510.1.5": 1_501_500_00, "DI510.2.2": 500_00, "DI510.2.5": 200_00}
	for ref, v := range want {
		if r.Values[ref] != v {
			t.Fatalf("%s = %d, want %d", ref, r.Values[ref], v)
		}
	}
	if PercentOfCapital(ex[1], r100.Values["DI100.31"]) != 2000 {
		t.Fatal("R2 000 of R10 000 is 20.00%")
	}
	if _, err := DI510("q", append(ex, Exposure{Person: "acme (pty) ltd ", Granted: 1}), r100, time.Now()); !errors.Is(err, ErrIncomplete) {
		t.Fatal("one person twice must be refused: exposures are per legal entity")
	}
}

func TestDI505IsDueOnlyWhenAnExposureCrossesTheThreshold(t *testing.T) {
	due, err := DI505Due(map[string]Cents{"a": 900, "b": 2000}, map[string]Cents{"a": 1100, "b": 2500, "c": 50}, 10_000, 1000)
	if err != nil || len(due) != 1 || due[0] != "a" {
		t.Fatalf("%v %v", due, err)
	}
	if _, err := DI505Due(nil, nil, 0, 1000); !errors.Is(err, ErrIncomplete) {
		t.Fatal("capital is required")
	}
}
