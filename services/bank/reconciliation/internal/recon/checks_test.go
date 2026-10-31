package recon

import (
	"strings"
	"testing"
)

// consistent builds a ZAR book that reconciles: customers hold 70, 30 is owed to the sponsor for an outbound
// payment not yet cleared, and the pool holds 100 received in one inbound movement.
func consistent() (Totals, Statement) {
	t := NewTotals()
	_ = t.Add(3003, 100, 0) // pool: debited 100 by the inbound credit
	_ = t.Add(1001, 0, 100) // customer: credited 100
	_ = t.Add(1001, 30, 0)  // customer: debited 30 for an outbound payment
	_ = t.Add(3001, 0, 30)  // owed to the sponsor for that payment
	s := Statement{Opening: 0, Closing: 100, InboundSum: 100}
	return t, s
}

func find(rs []Result, name string) Result {
	for _, r := range rs {
		if r.Check == name {
			return r
		}
	}
	return Result{}
}

func TestConsistentBookMatchesEveryCheck(t *testing.T) {
	tot, st := consistent()
	rs := Evaluate(ZAR, tot, st, InFlight{})
	if !AllMatched(rs) {
		t.Fatalf("consistent book should match: %+v", rs)
	}
	if len(rs) != 5 {
		t.Fatalf("want 5 checks, got %d", len(rs))
	}
}

func TestStatementArithmeticBreak(t *testing.T) {
	tot, st := consistent()
	st.Closing = 101
	r := find(Evaluate(ZAR, tot, st, InFlight{}), CheckStatementArithmetic)
	if r.Matched || r.Difference != 1 {
		t.Fatalf("want a break of 1, got %+v", r)
	}
}

func TestDoubleEntryBreakWhenAPostingIsOneSided(t *testing.T) {
	tot, st := consistent()
	_ = tot.Add(1001, 0, 5) // a one-sided credit with no matching debit
	r := find(Evaluate(ZAR, tot, st, InFlight{}), CheckLedgerDoubleEntry)
	if r.Matched || r.Difference != -5 {
		t.Fatalf("want a double-entry break of -5, got %+v", r)
	}
}

func TestUnclassifiedBalanceBreaks(t *testing.T) {
	tot, st := consistent()
	_ = tot.Add(9999, 0, 7)
	r := find(Evaluate(ZAR, tot, st, InFlight{}), CheckUnclassifiedBalance)
	if r.Matched || r.Difference != 7 || !strings.Contains(r.Detail, "9999") {
		t.Fatalf("want an unclassified break of 7 naming 9999, got %+v", r)
	}
}

func TestEquationBreakWhenStatementDisagreesWithLedger(t *testing.T) {
	tot, st := consistent()
	st.Closing = 90
	st.InboundSum = 90
	rs := Evaluate(ZAR, tot, st, InFlight{})
	if r := find(rs, "sponsor_equation"); r.Matched || r.Difference != -10 {
		t.Fatalf("want equation break -10, got %+v", r)
	}
	if r := find(rs, "sponsor_pool_mirror"); r.Matched || r.Difference != 10 {
		t.Fatalf("want pool mirror break +10, got %+v", r)
	}
}

// An inbound credit the bank has made but the ledger has not booked yet: the statement is ahead of the books by
// exactly that amount, and in-flight inbound explains it.
func TestInFlightInboundExplainsAStatementAhead(t *testing.T) {
	tot, st := consistent()
	st.Closing = 110
	st.InboundSum = 110
	if rs := Evaluate(ZAR, tot, st, InFlight{Inbound: 10}); !AllMatched(rs) {
		t.Fatalf("in-flight inbound should explain the difference: %+v", rs)
	}
	if rs := Evaluate(ZAR, tot, st, InFlight{}); AllMatched(rs) {
		t.Fatal("without in-flight inbound the same statement must break")
	}
}

// The bank has debited an outbound payment the ledger has not yet cleared. The statement is behind the books.
func TestInFlightOutboundExplainsAStatementBehind(t *testing.T) {
	tot, _ := consistent()
	st := Statement{Opening: 0, Closing: 70, InboundSum: 100, OutboundSum: 30}
	if rs := Evaluate(ZAR, tot, st, InFlight{Outbound: 30}); !AllMatched(rs) {
		t.Fatalf("in-flight outbound should explain the difference: %+v", rs)
	}
}

func TestUSDUsesThePartnerSourceName(t *testing.T) {
	tot := NewTotals()
	_ = tot.Add(5001, 500, 0)
	_ = tot.Add(1001, 0, 500)
	st := Statement{Closing: 500, InboundSum: 500}
	rs := Evaluate(USD, tot, st, InFlight{})
	if !AllMatched(rs) {
		t.Fatalf("USD mirror should match: %+v", rs)
	}
	if find(rs, "partner_equation").Check == "" || find(rs, "partner_pool_mirror").Check == "" {
		t.Fatalf("partner checks missing: %+v", rs)
	}
}

func TestEmptyResultsDoNotCountAsMatched(t *testing.T) {
	if AllMatched(nil) {
		t.Fatal("no results must not count as matched")
	}
}

func TestTotalsRefuseOverflow(t *testing.T) {
	tot := NewTotals()
	big := int64(1) << 62
	if err := tot.Add(1001, big, 0); err != nil {
		t.Fatal(err)
	}
	if err := tot.Add(1001, big, 0); err == nil {
		t.Fatal("a sum that overflows int64 must be refused, not wrapped")
	}
}

func TestByAssetRejectsUnknown(t *testing.T) {
	if _, ok := ByAsset("EUR"); ok {
		t.Fatal("EUR is not a v1 rail")
	}
	if r, ok := ByAsset("ZAR"); !ok || r.Ledger != 710 {
		t.Fatalf("ZAR = %+v, %v", r, ok)
	}
}
