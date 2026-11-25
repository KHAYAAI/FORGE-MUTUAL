package returns

import (
	"errors"
	"testing"
	"time"
)

func TestDI410MeasuresTheRepricingGapAndTheIncomeEffect(t *testing.T) {
	r100, _ := computeDI(t, book(), 0)
	items := []RepricingItem{
		{5, 2, 1_200_000_00}, // assets repricing within a month
		{7, 2, 1_030_000_00}, // demand and savings deposits and foreign funding
		{7, 3, 100_000_00},   // 32-day notice
		{7, 3, 50_000_00},    // 90-day notice
		{10, 6, -20_000_00},  // capital and other non-rate items, as the balancing negative gap
	}
	r, err := DI410("2027-12", DI410Inputs{DI100: r100, Items: items, Forecasts: map[int][4]int64{17: {1125, 1125, 1100, 1075}}}, time.Now())
	if err != nil || len(r.Breaks) != 0 {
		t.Fatalf("%v %v", r.Breaks, err)
	}
	// gap: 0-31 days +170 000; 32-91 days -150 000; other -20 000
	if r.Values["DI410.8.2"] != 170_000_00 || r.Values["DI410.8.3"] != -150_000_00 || r.Values["DI410.12.1"] != 0 || r.Values["DI410.17.4"] != 1075 {
		t.Fatalf("%v", r.Values)
	}
	// +1%: 170 000 x 1% x 11.5/12 - 150 000 x 1% x 10/12 = 1 629.17 - 1 250 = 379.17
	if r.Values["DI410.22"] != 379_16 && r.Values["DI410.22"] != 379_17 {
		t.Fatalf("income effect %d", r.Values["DI410.22"])
	}
	if r.Values["DI410.23"] != -r.Values["DI410.22"] || r.Values["DI410.24"] == 0 {
		t.Fatal("the fall mirrors the rise, and the effect is shown against capital")
	}
	items[0].Amount++
	r, _ = DI410("2027-12", DI410Inputs{DI100: r100, Items: items}, time.Now())
	if len(r.Breaks) == 0 {
		t.Fatal("rated and non-rate items that do not balance must break")
	}
	if _, err := DI410("2027-12", DI410Inputs{DI100: r100, Items: []RepricingItem{{3, 2, 1}}}, time.Now()); !errors.Is(err, ErrIncomplete) {
		t.Fatal("line 3 is computed, not an input")
	}
}
