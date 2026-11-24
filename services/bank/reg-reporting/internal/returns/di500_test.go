package returns

import (
	"errors"
	"testing"
	"time"
)

func di500In(t *testing.T) DI500Inputs {
	r100, _ := computeDI(t, book(), 0)
	loans := r100.Values["DI100.39"] // includes balances with banks
	return DI500Inputs{
		DI100: r100, PolicyMonths: 1,
		OverdueStart: 5_000_00, Recovered: 1_000_00, NewOverdue: 2_000_00, SpecificProvisions: 1_500_00, SecurityHeld: 500_00,
		WrittenOffIncome: 1_000_00, Overdue0to3: 4_000_00, Overdue3to6: 1_500_00, OverdueOver6: 500_00,
		Yields: []AssetYield{
			{Amount: loans - 320_000_00, YieldBps: 700},              // balances with banks
			{Amount: 300_000_00, YieldBps: 1450},                     // loans well above cost
			{Amount: 15_000_00, YieldBps: 1450, NonPerforming: true}, // overdue loans
			{Amount: 5_000_00, YieldBps: 400},                        // below the cost of funds
		},
		CostOfFundsBps: 500, OpexOverAssetsBps: 300,
	}
}

func TestDI500OverduesProfitabilityAndSectors(t *testing.T) {
	in := di500In(t)
	in.SectorAmounts = map[string]Cents{"services": in.DI100.Values["DI100.52"] - 20_000_00, "individuals": 20_000_00}
	in.SectorClients = map[string]Cents{"services": 12, "individuals": 3}
	r, err := DI500("2027-12", in, time.Now())
	if err != nil || len(r.Breaks) != 0 {
		t.Fatalf("%v %v", r.Breaks, err)
	}
	want := map[string]Cents{
		"DI500.6.3": 4_500_00, "DI500.8.3": 4_000_00, "DI500.9.4": 1_000_00, "DI500.24.4": 20_000_00, "DI500.24.2": 300_000_00,
		"DI500.26.12": in.DI100.Values["DI100.52"], "DI500.27.12": 15,
	}
	for ref, v := range want {
		if r.Values[ref] != v {
			t.Fatalf("%s = %d, want %d", ref, r.Values[ref], v)
		}
	}
	if r.Values["DI500.25.9"] != 10000 {
		t.Fatal("the percentages total 100%")
	}
}

func TestDI500CatchesFiguresThatDoNotAgree(t *testing.T) {
	in := di500In(t)
	in.OverdueOver6++
	if r, _ := DI500("2027-12", in, time.Now()); len(r.Breaks) == 0 {
		t.Fatal("an age analysis that does not add up must break")
	}
	in = di500In(t)
	in.Yields = in.Yields[1:]
	if r, _ := DI500("2027-12", in, time.Now()); len(r.Breaks) == 0 {
		t.Fatal("profitability columns that miss assets must break")
	}
	in = di500In(t)
	in.SectorAmounts = map[string]Cents{"fintech": 1}
	if _, err := DI500("2027-12", in, time.Now()); !errors.Is(err, ErrIncomplete) {
		t.Fatal("a sector that is not on the form must be refused")
	}
	in = di500In(t)
	in.PolicyMonths = 0
	if _, err := DI500("2027-12", in, time.Now()); !errors.Is(err, ErrIncomplete) {
		t.Fatal("line 1 is required")
	}
}
