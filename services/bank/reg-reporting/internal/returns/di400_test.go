package returns

import (
	"errors"
	"testing"
	"time"
)

func di400Inputs() DI400Inputs {
	off, _ := DI110("2027-Q4", DI110Inputs{UndrawnUnderOneYear: 50_000_00, UndrawnOneYearAndOver: 100_000_00}, time.Now())
	return DI400Inputs{
		AverageAssets:   map[int]Cents{40: 1_000_000_00, 47: 400_000_00, 74: 10_000_00, 36: 0},
		OffBalance:      off,
		CapitalRatioBps: 1000, PhaseInBps: 10000,
		MemberShares: 80_000_00, AppropriatedReserves: 5_000_00, GeneralProvisions: 2_000_00,
	}
}

func TestDI400WeightsAssetsAndComputesTheRatio(t *testing.T) {
	r, err := DI400("2027-Q4", di400Inputs(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	// 20% of 1 000 000 + 100% of 410 000 + 50% of 100 000 = 660 000 weighted
	want := map[string]Cents{
		"DI400.1.4": 1_000_000_00, "DI400.1.6": 410_000_00, "DI400.2.1": 50_000_00, "DI400.2.5": 100_000_00,
		"DI400.3.9": 1_560_000_00, "DI400.9.4": 200_000_00, "DI400.9.5": 50_000_00, "DI400.9.6": 410_000_00, "DI400.9.9": 660_000_00,
		"DI400.11": 66_000_00, "DI400.13": 66_000_00,
		"DI400.14": 85_000_00, "DI400.28": 2_000_00, "DI400.36": 2_000_00, "DI400.37": 87_000_00, "DI400.39": 87_000_00,
		"DI400.44": 1318, // 87 000 / 660 000 = 13.18%
	}
	for ref, v := range want {
		if r.Values[ref] != v {
			t.Fatalf("%s = %d, want %d", ref, r.Values[ref], v)
		}
	}
	if CapitalShortfall(r) != 0 {
		t.Fatal("no shortfall expected")
	}
}

// Profit not yet appropriated by the board is not capital; secondary capital cannot exceed primary.
func TestUnappropriatedProfitIsNotCapitalAndSecondaryIsCapped(t *testing.T) {
	in := di400Inputs()
	in.AppropriatedReserves = 0
	in.GeneralProvisions = 200_000_00
	r, _ := DI400("q", in, time.Now())
	if r.Values["DI400.14"] != 80_000_00 || r.Values["DI400.36"] != 80_000_00 || r.Values["DI400.40"] != 120_000_00 || r.Values["DI400.39"] != 160_000_00 {
		t.Fatalf("14=%d 36=%d 40=%d 39=%d", r.Values["DI400.14"], r.Values["DI400.36"], r.Values["DI400.40"], r.Values["DI400.39"])
	}
}

func TestACapitalShortfallIsReportedAndForcesAQualifiedDeclaration(t *testing.T) {
	in := di400Inputs()
	in.PreviousRequired = 100_000_00
	r, err := DI400("2027-Q4", in, time.Now())
	if err != nil || CapitalShortfall(r) != 13_000_00 || len(r.Breaks) != 0 {
		t.Fatalf("shortfall %d, breaks %v, %v", CapitalShortfall(r), r.Breaks, err)
	}
	if _, err := Prepare(r, "Naledi", "sha256:x"); err != nil {
		t.Fatal("a shortfall is reported, not blocked")
	}
	if _, err := Declare("2027-Q4", []Result{r}, nil, nil, "Thandi", "Pieter", time.Now()); !errors.Is(err, ErrIncomplete) {
		t.Fatal("an unqualified declaration over a shortfall must be refused")
	}
	d, err := Declare("2027-Q4", []Result{r}, nil, map[string]string{"DI 400": "short by R13 000 on every day from 2027-11-02"}, "Thandi", "Pieter", time.Now())
	if err != nil || d.Text == "" {
		t.Fatal(err)
	}
}

func TestDI400RefusesWhatItCannotWeight(t *testing.T) {
	in := di400Inputs()
	in.AverageAssets[99] = 1
	if _, err := DI400("q", in, time.Now()); !errors.Is(err, ErrIncomplete) {
		t.Fatal("an unknown DI 100 line must be refused")
	}
	in = di400Inputs()
	in.CapitalRatioBps = 0
	if _, err := DI400("q", in, time.Now()); !errors.Is(err, ErrIncomplete) {
		t.Fatal("the capital ratio is required")
	}
	// Every DI 100 asset leaf has a weighting.
	lines, _ := LoadForm("di100")
	for _, l := range lines {
		if l.Section == "ASSETS" && len(l.Sum) == 0 && l.No != 53 && l.No != 62 {
			if _, ok := DI100Weight[l.No]; !ok {
				t.Fatalf("DI 100 asset line %d %q has no risk weighting", l.No, l.Label)
			}
		}
	}
}

func TestDI110FeedsDI400AndTheOpenFXPositionIsWeightedInFull(t *testing.T) {
	off, err := DI110("2027-Q4", DI110Inputs{UndrawnUnderOneYear: 50_000_00, UndrawnOneYearAndOver: 100_000_00, NetOpenFX: 30_000_00}, time.Now())
	if err != nil || len(off.Breaks) != 0 || off.Values["DI110.11.1"] != 180_000_00 || off.Values["DI110.11.8"] != 30_000_00 || off.Values["DI110.11.7"] != 100_000_00 {
		t.Fatalf("%v %v", off.Breaks, err)
	}
	in := di400Inputs()
	in.OffBalance = off
	r, _ := DI400("2027-Q4", in, time.Now())
	if r.Values["DI400.2.6"] != 30_000_00 || r.Values["DI400.9.9"] != 690_000_00 {
		t.Fatalf("2.6=%d 9.9=%d", r.Values["DI400.2.6"], r.Values["DI400.9.9"])
	}
	in.OffBalance = Result{}
	if _, err := DI400("q", in, time.Now()); !errors.Is(err, ErrIncomplete) {
		t.Fatal("DI 400 needs the DI 110")
	}
	if _, err := DI110("q", DI110Inputs{NetOpenFX: -1}, time.Now()); !errors.Is(err, ErrIncomplete) {
		t.Fatal("negative amounts are refused")
	}
}
