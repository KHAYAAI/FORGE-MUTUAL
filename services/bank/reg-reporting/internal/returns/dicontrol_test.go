package returns

import (
	"encoding/csv"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestNilReturnsNeedACleanChartAndSometimesAPerson(t *testing.T) {
	now := time.Now()
	for form := range NilForms {
		_, err := Nil(form, "2027-12", book(), "Naledi", now)
		if err != nil {
			t.Fatalf("%s: %v", form, err)
		}
	}
	// An unknown account code might be a trading position, a security or a lease: no nil can be declared over it.
	withUnknown := append(book(), Position{"ZAR", 4100, "", 1})
	if _, err := Nil("DI 420", "2027-12", withUnknown, "", now); !errors.Is(err, ErrNotNil) {
		t.Fatalf("want ErrNotNil, got %v", err)
	}
	if _, err := Nil("DI 525", "2027-12", book(), " ", now); !errors.Is(err, ErrNeedsPerson) {
		t.Fatalf("DI 525 needs a person: %v", err)
	}
	if _, err := Nil("DI 100", "2027-12", book(), "x", now); !errors.Is(err, ErrNotNil) {
		t.Fatal("DI 100 is never a nil form")
	}
}

func TestTheDeclarationNeedsTwoOfficersCleanReturnsAndStatesEveryQualification(t *testing.T) {
	r100, r200 := computeDI(t, book(), 0)
	n, _ := Nil("DI 525", "2027-12", book(), "Naledi", time.Now())
	if _, err := Declare("2027-12", []Result{r100, r200}, nil, nil, "Thandi", "thandi", time.Now()); !errors.Is(err, ErrSameSigner) {
		t.Fatal("one person cannot sign as both officers")
	}
	d, err := Declare("2027-12", []Result{r100, r200}, []NilReturn{n}, nil, "Thandi", "Pieter", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d.Text, "FORM DI 100") || strings.Contains(d.Text, "FORM DI 310") || !strings.Contains(d.Text, "DI 525: no loan to a local government") ||
		!strings.Contains(d.Text, "(confirmed by Naledi)") || strings.Join(d.Forms, ",") != "DI 100,DI 200,DI 525" {
		t.Fatal(d.Text)
	}
	if _, err := Declare("2027-12", []Result{r100}, nil, map[string]string{"DI 600": "x"}, "Thandi", "Pieter", time.Now()); !errors.Is(err, ErrIncomplete) {
		t.Fatal("a qualification for a form not in the set is refused")
	}
	if _, err := Declare("2027-12", []Result{r100}, nil, map[string]string{"DI 100": " "}, "Thandi", "Pieter", time.Now()); !errors.Is(err, ErrIncomplete) {
		t.Fatal("a qualification needs the statement of the deficiency")
	}
	d, _ = Declare("2027-12", []Result{r100}, nil, map[string]string{"DI 100": "impairment of R1m on 2027-12-15"}, "Thandi", "Pieter", time.Now())
	if !strings.Contains(d.Text, "QUALIFIED: impairment of R1m") {
		t.Fatal(d.Text)
	}
	broken := r100
	broken.Breaks = []string{"x"}
	if _, err := Declare("2027-12", []Result{broken}, nil, nil, "Thandi", "Pieter", time.Now()); !errors.Is(err, ErrBreak) {
		t.Fatal("a set with a broken return cannot be declared")
	}
	if _, err := Declare("2028-01", []Result{r100}, nil, nil, "Thandi", "Pieter", time.Now()); !errors.Is(err, ErrIncomplete) {
		t.Fatal("a return for another period cannot be in the set")
	}
}

// The catalogue in docs and the code must agree on how each DI form is treated.
func TestTheCatalogueAndTheCodeAgree(t *testing.T) {
	f, err := os.Open("../../../../../docs/regulatory/returns/catalogue.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	built := map[string]bool{"DI 099": true, "DI 100": true, "DI 200": true, "DI 400": true, "DI 310": true, "DI 110": true, "DI 600": true, "DI 300": true, "DI 930": true, "DI 505": true, "DI 510": true, "DI 410": true, "DI 700": true, "DI 500": true}
	seen := map[string]bool{}
	di := 0
	for _, r := range rows[1:] {
		if r[0] != "mutual" {
			continue
		}
		di++
		form, status := r[1], r[8]
		seen[form] = true
		switch status {
		case "nil":
			if _, ok := NilForms[form]; !ok {
				t.Fatalf("%s is nil in the catalogue but not in NilForms", form)
			}
		case "not-applicable":
			if _, ok := NotApplicable[form]; !ok {
				t.Fatalf("%s is not applicable in the catalogue but not in NotApplicable", form)
			}
		case "built":
			if !built[form] {
				t.Fatalf("%s is built in the catalogue but has no builder", form)
			}
		case "to-build":
		default:
			t.Fatalf("%s: unknown status %q", form, status)
		}
	}
	if di != 27 {
		t.Fatalf("the catalogue lists %d DI forms; the directive lists 27", di)
	}
	for form := range NilForms {
		if !seen[form] {
			t.Fatalf("%s is in NilForms but not in the catalogue", form)
		}
	}
	for form := range built {
		if !seen[form] {
			t.Fatalf("%s has a builder but is not in the catalogue", form)
		}
	}
}
