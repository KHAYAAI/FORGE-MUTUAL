package returns

import (
	"errors"
	"testing"
	"time"
)

func TestDI700IsWithinTheLimitsWhenNothingIsRestrictedAndCatchesABreach(t *testing.T) {
	r100, _ := computeDI(t, book(), 0)
	r, err := DI700("2027-12", DI700Inputs{DI100: r100}, time.Now())
	if err != nil || r.Values["DI700.7"] != r100.Values["DI100.31"] || r.Values["DI700.15"] != r100.Values["DI100.16"]/10 || len(Shortfalls(r)) != 0 {
		t.Fatalf("%v %v %v", r.Values, Shortfalls(r), err)
	}
	r, _ = DI700("2027-12", DI700Inputs{DI100: r100, Property: r100.Values["DI100.31"] + 1}, time.Now())
	if len(Shortfalls(r)) != 1 {
		t.Fatalf("property above capital must be a shortfall: %v", Shortfalls(r))
	}
	if _, err := DI700("2028-12", DI700Inputs{DI100: r100}, time.Now()); !errors.Is(err, ErrIncomplete) {
		t.Fatal("DI 700 needs the same year's DI 100")
	}
}
