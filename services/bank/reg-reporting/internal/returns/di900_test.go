package returns

import (
	"strings"
	"testing"
	"time"
)

func TestDI900BalancesAndPlacesEachPartyOnItsSectorLine(t *testing.T) {
	ps := book()
	ps = append(ps, Position{"ZAR", 1001, "personal-zar-current", 5_000_00}, Position{"ZAR", 3003, "system:pool:ZAR:v1", -5_000_00})
	in, problems := DI900Inputs(ps, 30_000_00, 0)
	if len(problems) != 0 {
		t.Fatal(problems)
	}
	s, err := DI900()
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.Compute("2027-12", in, time.Now())
	if err != nil || len(r.Breaks) != 0 {
		t.Fatalf("%v %v", r.Breaks, err)
	}
	r100, _ := computeDI(t, ps, 0)
	want := map[string]Cents{
		"DI900.19": 1_000_000_00, "DI900.21": 5_000_00, "DI900.32": 180_000_00, "DI900.27": 180_000_00, "DI900.1": 1_185_000_00,
		"DI900.174": 20_000_00, "DI900.179": 300_000_00, "DI900.153": 180_000_00, "DI900.86": 30_000_00,
		"DI900.74": 10_000_00, "DI900.77": 17_500_00,
		"DI900.80": r100.Values["DI100.34"], "DI900.223": r100.Values["DI100.76"],
	}
	for ref, v := range want {
		if r.Values[ref] != v {
			t.Fatalf("%s = %d, want %d", ref, r.Values[ref], v)
		}
	}
	lines, _ := LoadForm("di900")
	if len(lines) != 229 || lines[len(lines)-1].No != 230 {
		t.Fatalf("%d lines", len(lines))
	}
	if tab := FormTable(s, r); !strings.Contains(tab, "| 230 |") || !strings.Contains(tab, "| Individuals | 21 | 5 |") {
		t.Fatal("the table must show the form's own line numbers")
	}
}
