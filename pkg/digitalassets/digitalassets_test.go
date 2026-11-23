package digitalassets

import (
	"errors"
	"testing"
	"time"
)

func TestReservesMustCoverEveryTokenAndAShortfallOfOneHalts(t *testing.T) {
	a, err := Attest(1_000_000, 1_000_000)
	if err != nil || !a.FullyBacked || a.Halt || a.SurplusMinor != 0 {
		t.Fatalf("exactly backed: %+v %v", a, err)
	}
	a, _ = Attest(1_000_000, 1_250_000)
	if !a.FullyBacked || a.Halt || a.SurplusMinor != 250_000 {
		t.Fatalf("a surplus is allowed and reported: %+v", a)
	}
	a, _ = Attest(1_000_000, 999_999)
	if a.FullyBacked || !a.Halt || a.SurplusMinor != -1 {
		t.Fatalf("one unit short must halt: %+v", a)
	}
	if _, err := Attest(-1, 0); !errors.Is(err, ErrBadAmount) {
		t.Fatal("negative amounts are refused")
	}
	if a, _ := Attest(0, 0); !a.FullyBacked || a.Halt {
		t.Fatal("nothing issued is fully backed")
	}
}

func TestAWalletIsUsableOnlyWhenEveryCheckPasses(t *testing.T) {
	now := time.Date(2027, 3, 1, 12, 0, 0, 0, time.UTC)
	good := Wallet{CustomerID: "c1", Chain: "ethereum", Address: "0x52908400098527886E0F7030069857D2E4169EE7", OwnershipProof: "0xsig",
		ScreenedAt: now.Add(-time.Hour), Clear: true}
	if err := good.Usable(now); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		mut  func(*Wallet)
		want error
	}{
		{"unsupported chain", func(w *Wallet) { w.Chain = "bitcoin" }, ErrChain},
		{"short address", func(w *Wallet) { w.Address = "0x1234" }, ErrAddress},
		{"zero address", func(w *Wallet) { w.Address = "0x0000000000000000000000000000000000000000" }, ErrAddress},
		{"no proof of control", func(w *Wallet) { w.OwnershipProof = " " }, ErrNoProof},
		{"never screened", func(w *Wallet) { w.ScreenedAt = time.Time{} }, ErrNotScreened},
		{"stale screening", func(w *Wallet) { w.ScreenedAt = now.Add(-25 * time.Hour) }, ErrNotScreened},
		{"failed screening", func(w *Wallet) { w.Clear = false }, ErrNotClear},
	}
	for _, c := range cases {
		w := good
		c.mut(&w)
		if err := w.Usable(now); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", c.name, err, c.want)
		}
	}
}

func TestTravelRuleNeedsMoreAboveTheThreshold(t *testing.T) {
	full := Party{Name: "Thandi Dube", Account: "0xabc", Identifier: "8001010012345"}
	bene := Party{Name: "Acme Exchange", Account: "0xdef"}
	if err := TravelRule(500, Party{Name: "Thandi Dube", Account: "0xabc"}, bene); err != nil {
		t.Fatalf("below the threshold the names and accounts are enough: %v", err)
	}
	if err := TravelRule(1000, Party{Name: "Thandi Dube", Account: "0xabc"}, bene); !errors.Is(err, ErrTravelRule) {
		t.Fatalf("at the threshold the originator's identifier is needed: %v", err)
	}
	if err := TravelRule(1000, full, bene); err != nil {
		t.Fatal(err)
	}
	if err := TravelRule(1, full, Party{Name: "", Account: "0xdef"}); !errors.Is(err, ErrTravelRule) {
		t.Fatal("a nameless beneficiary is refused")
	}
}
