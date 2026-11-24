package returns

import (
	"errors"
	"testing"
	"time"
)

func di310Base(t *testing.T) DI310Inputs {
	r100, _ := computeDI(t, book(), 0)
	return DI310Inputs{DI100: r100, ReserveRateBps: 250, LiquidRateBps: 500}
}

// The platform as built holds no liquid asset the Act recognises: a balance with the sponsor is not one. DI 310 must say so.
func TestASponsorBalanceIsNotALiquidAssetAndTheShortfallIsReported(t *testing.T) {
	in := di310Base(t)
	r, err := DI310("2027-12", in, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	liab := in.DI100.Values["DI100.34"] - in.DI100.Values["DI100.23"]
	if r.Values["DI310.3"] != liab || r.Values["DI310.11"] != liab*500/10000 || r.Values["DI310.12"] != 0 || r.Values["DI310.8"] != liab*250/10000 {
		t.Fatalf("%v", r.Values)
	}
	s := Shortfalls(r)
	if s["liquid assets"] != liab*500/10000 || s["minimum reserve balance"] != liab*250/10000 {
		t.Fatalf("shortfalls %v", s)
	}
	if _, err := Declare("2027-12", []Result{in.DI100, r}, nil, nil, "Thandi", "Pieter", time.Now()); !errors.Is(err, ErrIncomplete) {
		t.Fatal("a DI 310 shortfall must be declared")
	}
}

func TestHoldingTreasuryBillsAndAReserveBalanceMeetsTheMinimums(t *testing.T) {
	in := di310Base(t)
	liab := in.DI100.Values["DI100.34"] - in.DI100.Values["DI100.23"]
	in.Liquid = map[int]Cents{15: liab * 100 / 10000, 16: liab * 400 / 10000}
	in.ReserveBalanceHeld = liab * 250 / 10000
	r, err := DI310("2027-12", in, time.Now())
	if err != nil || len(Shortfalls(r)) != 0 || r.Values["DI310.12"] != liab*500/10000 {
		t.Fatalf("%v %v", Shortfalls(r), err)
	}
	in.Liquid = map[int]Cents{12: 1}
	if _, err := DI310("2027-12", in, time.Now()); !errors.Is(err, ErrIncomplete) {
		t.Fatal("line 12 is a total, not a liquid-asset class")
	}
	in = di310Base(t)
	if _, err := DI310("2028-01", in, time.Now()); !errors.Is(err, ErrIncomplete) {
		t.Fatal("DI 310 needs the same month's DI 100")
	}
}
