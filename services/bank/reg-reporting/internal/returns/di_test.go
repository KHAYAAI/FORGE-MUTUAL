package returns

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestTheTranscribedFormsAreCompleteAndTheirTotalsResolve(t *testing.T) {
	for file, n := range map[string]int{"di100": 76, "di200": 66} {
		lines, err := LoadForm(file)
		if err != nil {
			t.Fatal(err)
		}
		if len(lines) != n {
			t.Fatalf("%s has %d lines, the form has %d", file, len(lines), n)
		}
		for i, l := range lines {
			if l.No != i+1 || strings.TrimSpace(l.Label) == "" {
				t.Fatalf("%s line %d: %+v", file, i+1, l)
			}
			for _, term := range l.Sum {
				if term.From < 1 || term.To > n || (term.From <= l.No && l.No <= term.To) {
					t.Fatalf("%s line %d: bad term %+v", file, l.No, term)
				}
			}
		}
	}
	// A label that says "total of items" must have a sum, and the reverse.
	for _, file := range []string{"di100", "di200"} {
		lines, _ := LoadForm(file)
		for _, l := range lines {
			says := strings.Contains(strings.ToLower(l.Label), "total of items") || strings.Contains(l.Label, " less item") || strings.Contains(l.Label, "plus item")
			if says != (len(l.Sum) > 0) && !(file == "di200" && l.No == 64) {
				t.Fatalf("%s line %d %q: label and arithmetic disagree", file, l.No, l.Label)
			}
		}
	}
}

// A small balanced book in rand: current, savings and notice deposits, a USD deposit, loans, cash at the sponsor and the
// USD partner, interest, fees (some from FX) and a write-off.
func book() []Position {
	ps := []Position{
		{"ZAR", 1001, "business-zar-current", 600_000_00},
		{"ZAR", 1001, "savings", 250_000_00},
		{"ZAR", 1001, "notice-32", 100_000_00},
		{"ZAR", 1001, "notice-90", 50_000_00},
		{"USD", 1001, "usd-current", 180_000_00},
		{"USD", 5001, "", -180_000_00},
		{"ZAR", 1020, "working-capital", -300_000_00},
		{"ZAR", 1010, "overdraft", -20_000_00},
		{"ZAR", 2008, "", 20_000_00},
		{"ZAR", 2007, "savings", -3_000_00},
		{"ZAR", 2007, "notice-32", -2_000_00},
		{"ZAR", 2002, "system:fees:ZAR:v1", 2_000_00},
		{"ZAR", 2002, "system:fx-fees:ZAR:v1", 1_500_00},
		{"ZAR", 2006, "", -1_000_00},
		{"ZAR", 2005, "", 0},
		{"ZAR", 8001, "", 10_000_00},
	}
	var rest Cents
	for _, p := range ps {
		rest += p.Net
	}
	return append(ps, Position{"ZAR", 3003, "system:pool:ZAR:v1", -rest})
}

func computeDI(t *testing.T, ps []Position, opening Cents) (Result, Result) {
	t.Helper()
	in, problems := DIInputs(ps, opening)
	if len(problems) != 0 {
		t.Fatalf("problems: %v", problems)
	}
	s100, err := DI100()
	if err != nil {
		t.Fatal(err)
	}
	s200, err := DI200()
	if err != nil {
		t.Fatal(err)
	}
	r100, err := s100.Compute("2027-12", in, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	r200, err := s200.Compute("2027-12", in, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return r100, r200
}

func TestDI100BalancesAndPlacesEachBalanceOnTheFormsLine(t *testing.T) {
	r, r200 := computeDI(t, book(), 0)
	if len(r.Breaks) != 0 {
		t.Fatalf("breaks: %v", r.Breaks)
	}
	want := map[string]Cents{
		"DI100.5": 600_000_00, "DI100.6": 250_000_00, "DI100.7": 150_000_00, "DI100.4": 1_000_000_00,
		"DI100.11": 180_000_00, "DI100.9": 180_000_00, "DI100.16": 1_180_000_00,
		"DI100.24": 10_000_00, "DI100.29": 10_000_00, "DI100.31": 10_000_00,
		"DI100.32": 17_500_00, // 20 000 - 5 000 + 3 500 - 1 000
		"DI100.47": 320_000_00, "DI100.52": 320_000_00 + 180_000_00 + (1_180_000_00 + 27_500_00 - 320_000_00 - 180_000_00),
	}
	for ref, v := range want {
		if r.Values[ref] != v {
			t.Fatalf("%s = %d, want %d", ref, r.Values[ref], v)
		}
	}
	if r.Values["DI100.76"] != r.Values["DI100.34"] || r.Values["DI100.76"] == 0 {
		t.Fatalf("assets %d, liabilities %d", r.Values["DI100.76"], r.Values["DI100.34"])
	}
	if br := CrossCheck(r, r200); br != "" {
		t.Fatal(br)
	}
}

func TestDI200FollowsTheFormsArithmetic(t *testing.T) {
	_, r := computeDI(t, book(), 0)
	want := map[string]Cents{
		"DI200.8": 20_000_00, "DI200.1": 20_000_00, "DI200.15": 3_000_00, "DI200.16": 2_000_00, "DI200.11": 5_000_00,
		"DI200.22": 5_000_00, "DI200.23": 15_000_00, "DI200.27": 1_500_00, "DI200.24": 1_500_00, "DI200.35": 16_500_00,
		"DI200.37": 2_000_00, "DI200.36": 2_000_00, "DI200.42": 1_000_00, "DI200.39": 1_000_00, "DI200.51": 17_500_00,
		"DI200.57": 17_500_00, "DI200.64": 17_500_00, "DI200.66": 17_500_00,
	}
	for ref, v := range want {
		if r.Values[ref] != v {
			t.Fatalf("%s = %d, want %d", ref, r.Values[ref], v)
		}
	}
}

// Item 45 is "total of items 46 to 50", but 46 already totals 47 and 48: summing 46 to 50 would count them twice.
func TestProvisionsAreNotCountedTwice(t *testing.T) {
	s, _ := DI200()
	in, _ := DIInputs(nil, 0)
	in["DI200.47"], in["DI200.48"], in["DI200.49"], in["DI200.50"] = 100, 10, 1000, 10000
	r, err := s.Compute("p", in, time.Now())
	if err != nil || r.Values["DI200.46"] != 110 || r.Values["DI200.45"] != 11110 {
		t.Fatalf("46=%d 45=%d %v", r.Values["DI200.46"], r.Values["DI200.45"], err)
	}
}

func TestOpeningRetainedIncomeCarriesToDI100(t *testing.T) {
	// A ledger that has run since before this year: DI 100 line 32 holds all profit to date, DI 200 the year plus the opening.
	ps := book()
	ps = append(ps, Position{"ZAR", 2002, "system:fees:ZAR:v1", 4_000_00}, Position{"ZAR", 3003, "system:pool:ZAR:v1", -4_000_00})
	in, problems := DIInputs(ps, 0)
	if len(problems) != 0 {
		t.Fatal(problems)
	}
	// Report the year's movement only: subtract last year's 4 000 from DI 200 and give it as the opening balance.
	in["DI200.37"] -= 4_000_00
	in["DI200.65"] = 4_000_00
	s100, _ := DI100()
	s200, _ := DI200()
	r100, _ := s100.Compute("p", in, time.Now())
	r200, _ := s200.Compute("p", in, time.Now())
	if br := CrossCheck(r100, r200); br != "" {
		t.Fatal(br)
	}
	in["DI200.65"] = 0
	r200, _ = s200.Compute("p", in, time.Now())
	if CrossCheck(r100, r200) == "" {
		t.Fatal("a missing opening balance must show as a break between the forms")
	}
}

func TestPositionsWithNoPlaceOnTheFormAreProblems(t *testing.T) {
	for name, p := range map[string]Position{
		"unknown code":                   {"ZAR", 9999, "", 1},
		"unclassed deposit product":      {"ZAR", 1001, "mystery", 1},
		"interest expense with no split": {"ZAR", 2007, "", -1},
	} {
		_, problems := DIInputs([]Position{p}, 0)
		if len(problems) != 1 || !strings.Contains(problems[0], ErrUnmapped.Error()) {
			t.Fatalf("%s: %v", name, problems)
		}
	}
	// One cent of deposits with no other side: the sheet does not balance.
	ps := append(book(), Position{"ZAR", 1001, "savings", 1})
	in, _ := DIInputs(ps, 0)
	s, _ := DI100()
	r, _ := s.Compute("p", in, time.Now())
	if len(r.Breaks) == 0 {
		t.Fatal("an unbalanced ledger must break DI 100")
	}
	if _, err := Prepare(r, "Naledi", "sha256:x"); !errors.Is(err, ErrBreak) {
		t.Fatal("a return with a break cannot be prepared")
	}
}

func TestEdgeCasesGoToTheRightSide(t *testing.T) {
	in, problems := DIInputs([]Position{
		{"ZAR", 3001, "", 500_00},     // owed to the sponsor for EFT settlement: interbank funding
		{"USD", 3010, "", 200_00},     // owed to the AD partner: foreign funding from banks
		{"ZAR", 2003, "", -300_00},    // FX pool in debit: other assets
		{"ZAR", 3030, "", 400_00},     // card scheme settlement owed: other liabilities
		{"USD", 1020, "x", -1_000_00}, // a foreign-currency loan
		{"USD", 2008, "", 50_00},
	}, 0)
	if len(problems) != 0 {
		t.Fatal(problems)
	}
	for ref, v := range map[string]Cents{"DI100.3": 500_00, "DI100.10": 200_00, "DI100.74": 300_00, "DI100.22": 400_00, "DI100.48": 1_000_00, "DI200.9": 50_00} {
		if in[ref] != v {
			t.Fatalf("%s = %d, want %d", ref, in[ref], v)
		}
	}
}

func TestNoDIReturnIsSubmittableUntilAPersonChecksTheTranscription(t *testing.T) {
	r, _ := computeDI(t, book(), 0)
	if len(r.Unverified) != 76 {
		t.Fatalf("%d unverified lines", len(r.Unverified))
	}
	c, err := Prepare(r, "Naledi", "sha256:x")
	if err != nil {
		t.Fatal(err)
	}
	_ = c.Approve("Pieter", time.Now())
	_ = c.AuditorReport("Audit Co", "Partner", time.Now())
	if err := c.Submittable(); !errors.Is(err, ErrUnverified) {
		t.Fatalf("want ErrUnverified, got %v", err)
	}
}

func TestThousandsAndTheFormTable(t *testing.T) {
	for c, want := range map[Cents]int64{0: 0, 49_999: 0, 50_000: 1, 149_999: 1, 150_000: 2, -50_000: -1, 123_456_789: 1235} {
		if got := Thousands(c); got != want {
			t.Fatalf("Thousands(%d) = %d, want %d", c, got, want)
		}
	}
	r, _ := computeDI(t, book(), 0)
	s, _ := DI100()
	tab := FormTable(s, r)
	if !strings.Contains(tab, "| Demand | 5 | 600 |") || !strings.Contains(tab, "**TOTAL ASSETS") || !strings.Contains(tab, "Not computed:") {
		t.Fatal(tab[:400])
	}
}
