package returns

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// A small balanced ledger: R1m deposited, R300k lent out, R20k interest earned, R5k interest paid, R2k fees,
// R1k written off. Credits less debits, per code; the nets sum to zero.
func ledger() map[uint32]int64 {
	nets := map[uint32]int64{
		1001: 1_000_000_00, // deposits (credit)
		1020: -300_000_00,  // loans (debit)
		2008: 20_000_00,    // interest income
		2007: -5_000_00,    // interest expense
		2002: 2_000_00,     // fees
		2006: -1_000_00,    // written off
		2005: 0,            // suspense
	}
	var rest int64
	for _, v := range nets {
		rest += v
	}
	nets[3003] = -rest // the sponsor pooled account balances the books
	return nets
}

func TestBothFormsBalanceFromTheLedger(t *testing.T) {
	for _, form := range []LegalForm{Mutual, Bank} {
		in, unmapped := Inputs(ledger())
		if len(unmapped) != 0 {
			t.Fatalf("unmapped %v", unmapped)
		}
		bs, _ := BalanceSheet(form)
		r, err := bs.Compute("2027-12", in, time.Now())
		if err != nil || len(r.Breaks) != 0 {
			t.Fatalf("%s: %v %v", form, err, r.Breaks)
		}
		if r.Values["BS.3"] != r.Values["BS.7"] || r.Values["BS.2"] != 300_000_00 || r.Values["BS.6"] != 16_000_00 {
			t.Fatalf("%s balance sheet %v", form, r.Values)
		}
		is, _ := IncomeStatement(form)
		ri, err := is.Compute("2027-12", in, time.Now())
		if err != nil || len(ri.Breaks) != 0 || ri.Values["IS.6"] != 16_000_00 || ri.Values["IS.3"] != 15_000_00 {
			t.Fatalf("%s income statement %v %v %v", form, ri.Values, ri.Breaks, err)
		}
	}
	if bs, _ := BalanceSheet(Mutual); bs.Form != "DI 100" {
		t.Fatal(bs.Form)
	}
	if bs, _ := BalanceSheet(Bank); bs.Form != "BA 100" {
		t.Fatal(bs.Form)
	}
}

func TestAnUnbalancedLedgerOrAnUnmappedCodeIsCaught(t *testing.T) {
	nets := ledger()
	nets[1001] += 1 // one cent of deposits with no other side
	in, _ := Inputs(nets)
	bs, _ := BalanceSheet(Bank)
	r, _ := bs.Compute("2027-12", in, time.Now())
	if len(r.Breaks) == 0 {
		t.Fatal("an unbalanced ledger must break the balance sheet")
	}
	nets = ledger()
	nets[9999] = 5
	if _, unmapped := Inputs(nets); len(unmapped) != 1 || unmapped[0] != 9999 {
		t.Fatalf("unmapped code not reported: %v", unmapped)
	}
}

func TestAnAuditedReturnNeedsAnIndependentAuditor(t *testing.T) {
	in, _ := Inputs(ledger())
	bs, _ := BalanceSheet(Mutual)
	r, _ := bs.Compute("2027-12", in, time.Now())
	c, err := Prepare(r, "controller", "sha256:abc")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.AuditorReport("Firm", "partner", time.Now()); !errors.Is(err, ErrNotApproved) {
		t.Fatalf("audit before approval: %v", err)
	}
	_ = c.Approve("cfo", time.Now())
	if err := c.AuditorReport("Firm", "cfo", time.Now()); err == nil {
		t.Fatal("the approver cannot audit their own return")
	}
	if err := c.Submittable(); !errors.Is(err, ErrNotAudited) {
		t.Fatalf("unaudited: %v", err)
	}
	_ = c.AuditorReport("Audit Firm Inc", "A Partner", time.Now())
	if err := c.Submittable(); !errors.Is(err, ErrUnverified) {
		t.Fatalf("lines are still unverified against the form instructions: %v", err)
	}
	if !strings.Contains(Table(bs, r), "| BS.3 Total assets | 1 016 000.00 |") {
		t.Fatal(Table(bs, r))
	}
}
