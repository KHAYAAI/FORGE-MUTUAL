package banklayer

import "testing"

func TestBothLegalFormsAreDescribed(t *testing.T) {
	m, err := ProfileFor(LegalFormMutual)
	if err != nil || m.MinInitialCapitalMinor != 1_000_000_000 || m.MinFounders != 7 || !m.MemberShares || m.FitAndProperForm != "DI 020" {
		t.Fatalf("mutual = %+v, %v", m, err)
	}
	b, err := ProfileFor(LegalFormBank)
	if err != nil || b.MinInitialCapitalMinor != 25_000_000_000 || b.MemberShares || !b.OutsideEquity || b.FitAndProperForm != "BA 020" {
		t.Fatalf("bank = %+v, %v", b, err)
	}
	if _, err := ProfileFor(LegalFormNone); err == nil {
		t.Fatal("a sponsor-led entity has no own-licence profile")
	}
	b.Returns[0] = "changed"
	if again, _ := ProfileFor(LegalFormBank); again.Returns[0] != "BA 100" {
		t.Fatal("callers must not be able to change the table")
	}
}

func TestParseLegalFormDefaultsToTheMutual(t *testing.T) {
	for in, want := range map[string]LegalForm{"": LegalFormMutual, "mutual": LegalFormMutual, " Bank ": LegalFormBank, "LEGAL_FORM_BANK": LegalFormBank} {
		if got, err := ParseLegalForm(in); err != nil || got != want {
			t.Fatalf("%q = %v, %v", in, got, err)
		}
	}
	if _, err := ParseLegalForm("building society"); err == nil {
		t.Fatal("unknown forms are refused")
	}
	c, _ := OwnLicenceCapabilities(LegalFormMutual)
	if !c.MemberShares || c.LegalBookOfRecord != BookForge {
		t.Fatalf("mutual capabilities = %+v", c)
	}
}
