// Package contract is the behaviour suite every BankLayer implementation must pass (ARCHITECTURE.md 5.2, rule 2).
// It uses only the standard library. Any implementation that cannot satisfy a case is wrong, not the case.
package contract

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/khayaai/forge-mutual/pkg/banklayer"
)

// Harness is one bank under test. Inbound simulates a payer's credit arriving; a real bank cannot be asked for one,
// so the harness must be backed by a simulator (in-process or over HTTP).
type Harness struct {
	Bank    banklayer.BankLayer
	Inbound func(asset string, minor int64, reference string, partner bool) error
}

// Factory returns a fresh harness for each top-level test, so cases do not depend on each other's state.
type Factory func(t *testing.T) Harness

var seq atomic.Int64

func newID(prefix string) string {
	return fmt.Sprintf("%s-%d-%d", prefix, time.Now().UnixNano(), seq.Add(1))
}

// Run executes every case against harnesses from newHarness.
func Run(t *testing.T, newHarness Factory) {
	t.Helper()
	cases := []struct {
		name string
		fn   func(t *testing.T, h Harness)
	}{
		{"capabilities_describe_the_sponsor_route", capabilitiesSponsor},
		{"open_account_validates_and_issues_unique_references", openAccount},
		{"close_account_is_idempotent_and_refuses_unknown_accounts", closeAccount},
		{"pay_is_idempotent_and_refuses_reused_ids", payIdempotent},
		{"pay_rejects_bad_beneficiary_details_without_error", payRejectsBadDetails},
		{"pay_validates_the_request", payValidates},
		{"status_settles_once_and_books_one_debit", payStatusSettlesOnce},
		{"unknown_payment_is_not_found", payUnknown},
		{"inbound_credit_appears_in_statement_and_inbound_list", inboundCredit},
		{"statement_closing_equals_opening_plus_movements", statementArithmetic},
		{"usd_partner_books_are_separate_from_the_sponsor_pool", partnerSeparate},
		{"usd_partner_payout_settles_on_partner_books", partnerPayout},
		{"deposit_insurance_and_prudential_duties_match_route", dutiesMatchRoute},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.fn(t, newHarness(t))
		})
	}
}

func today() time.Time { return time.Now().UTC() }

func capabilitiesSponsor(t *testing.T, h Harness) {
	c := h.Bank.Capabilities()
	if c.Route != banklayer.RouteSponsor {
		t.Fatalf("route = %q, want %q", c.Route, banklayer.RouteSponsor)
	}
	if c.LegalForm != banklayer.LegalFormNone || c.LegalBookOfRecord != banklayer.BookSponsor {
		t.Fatalf("sponsor route must have no legal form and the sponsor as book of record, got %+v", c)
	}
	if c.DirectClearing || c.TokenisedDeposits || c.MemberShares {
		t.Fatalf("own-licence capabilities must be off on the sponsor route, got %+v", c)
	}
}

func openAccount(t *testing.T, h Harness) {
	ctx := context.Background()
	for _, bad := range []banklayer.OpenAccountRequest{{Owner: "", Asset: "ZAR"}, {Owner: "o", Asset: ""}} {
		if _, err := h.Bank.OpenAccount(ctx, bad); !errors.Is(err, banklayer.ErrInvalid) {
			t.Fatalf("OpenAccount(%+v) err = %v, want ErrInvalid", bad, err)
		}
	}
	seen := map[string]bool{}
	for i := 0; i < 3; i++ {
		acc, err := h.Bank.OpenAccount(ctx, banklayer.OpenAccountRequest{Owner: newID("owner"), Asset: "ZAR"})
		if err != nil {
			t.Fatalf("OpenAccount: %v", err)
		}
		if acc.ID == "" || acc.Reference == "" || acc.Asset != "ZAR" {
			t.Fatalf("account missing fields: %+v", acc)
		}
		if seen[acc.Reference] {
			t.Fatalf("reference %q issued twice", acc.Reference)
		}
		seen[acc.Reference] = true
	}
}

func closeAccount(t *testing.T, h Harness) {
	ctx := context.Background()
	acc, err := h.Bank.OpenAccount(ctx, banklayer.OpenAccountRequest{Owner: newID("owner"), Asset: "ZAR"})
	if err != nil {
		t.Fatalf("OpenAccount: %v", err)
	}
	if err := h.Bank.CloseAccount(ctx, acc.ID, "customer request"); err != nil {
		t.Fatalf("CloseAccount: %v", err)
	}
	if err := h.Bank.CloseAccount(ctx, acc.ID, "customer request"); err != nil {
		t.Fatalf("second CloseAccount must be a no-op, got %v", err)
	}
	if err := h.Bank.CloseAccount(ctx, "does-not-exist", "x"); !errors.Is(err, banklayer.ErrNotFound) {
		t.Fatalf("unknown account err = %v, want ErrNotFound", err)
	}
	if err := h.Bank.CloseAccount(ctx, acc.ID, ""); !errors.Is(err, banklayer.ErrInvalid) {
		t.Fatalf("empty reason err = %v, want ErrInvalid", err)
	}
}

func goodPay(id string) banklayer.PayRequest {
	return banklayer.PayRequest{
		ID: id, Asset: "ZAR", AmountMinor: 12345, Beneficiary: "Supplier One",
		AccountNumber: "1234567890", BankCode: "250655", Reference: "INV-1",
	}
}

func payIdempotent(t *testing.T, h Harness) {
	ctx := context.Background()
	req := goodPay(newID("pay"))
	first, err := h.Bank.Rails().Pay(ctx, req)
	if err != nil {
		t.Fatalf("Pay: %v", err)
	}
	again, err := h.Bank.Rails().Pay(ctx, req)
	if err != nil {
		t.Fatalf("retry of the same request must succeed, got %v", err)
	}
	if again.Status != first.Status || again.ID != first.ID {
		t.Fatalf("retry returned %+v, first returned %+v", again, first)
	}
	changed := req
	changed.AmountMinor++
	if _, err := h.Bank.Rails().Pay(ctx, changed); !errors.Is(err, banklayer.ErrConflict) {
		t.Fatalf("reused id with a different amount err = %v, want ErrConflict", err)
	}
}

func payRejectsBadDetails(t *testing.T, h Harness) {
	req := goodPay(newID("pay"))
	req.AccountNumber = "not-a-number"
	res, err := h.Bank.Rails().Pay(context.Background(), req)
	if err != nil {
		t.Fatalf("a bank refusal is a status, not an error: %v", err)
	}
	if res.Status != banklayer.StatusRejected {
		t.Fatalf("status = %q, want REJECTED", res.Status)
	}
}

func payValidates(t *testing.T, h Harness) {
	ctx := context.Background()
	for name, mutate := range map[string]func(*banklayer.PayRequest){
		"no id":          func(r *banklayer.PayRequest) { r.ID = "" },
		"zero amount":    func(r *banklayer.PayRequest) { r.AmountMinor = 0 },
		"negative":       func(r *banklayer.PayRequest) { r.AmountMinor = -1 },
		"no beneficiary": func(r *banklayer.PayRequest) { r.Beneficiary = "" },
		"no asset":       func(r *banklayer.PayRequest) { r.Asset = "" },
	} {
		req := goodPay(newID("pay"))
		mutate(&req)
		if _, err := h.Bank.Rails().Pay(ctx, req); !errors.Is(err, banklayer.ErrInvalid) {
			t.Fatalf("%s: err = %v, want ErrInvalid", name, err)
		}
	}
}

func payStatusSettlesOnce(t *testing.T, h Harness) {
	ctx := context.Background()
	req := goodPay(newID("pay"))
	before, err := h.Bank.StatementSource().Statement(ctx, "ZAR", today())
	if err != nil {
		t.Fatalf("Statement: %v", err)
	}
	if _, err := h.Bank.Rails().Pay(ctx, req); err != nil {
		t.Fatalf("Pay: %v", err)
	}
	steps := []string{banklayer.StatusPending, banklayer.StatusSettled, banklayer.StatusSettled, banklayer.StatusSettled}
	for i, want := range steps {
		got, err := h.Bank.Rails().Status(ctx, req.ID)
		if err != nil {
			t.Fatalf("Status %d: %v", i, err)
		}
		if got.Status != want {
			t.Fatalf("poll %d status = %q, want %q", i, got.Status, want)
		}
	}
	after, err := h.Bank.StatementSource().Statement(ctx, "ZAR", today())
	if err != nil {
		t.Fatalf("Statement: %v", err)
	}
	if after.ClosingMinor != before.ClosingMinor-req.AmountMinor {
		t.Fatalf("closing = %d, want %d (one debit, not one per poll)", after.ClosingMinor, before.ClosingMinor-req.AmountMinor)
	}
}

func payUnknown(t *testing.T, h Harness) {
	if _, err := h.Bank.Rails().Status(context.Background(), newID("never")); !errors.Is(err, banklayer.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func inboundCredit(t *testing.T, h Harness) {
	ctx := context.Background()
	ref := newID("FORGE")
	start := today().Add(-time.Second)
	if err := h.Inbound("ZAR", 50000, ref, false); err != nil {
		t.Fatalf("inject: %v", err)
	}
	list, err := h.Bank.StatementSource().Inbound(ctx, "ZAR", start)
	if err != nil {
		t.Fatalf("Inbound: %v", err)
	}
	found := false
	for _, m := range list {
		if m.Direction == "IN" && m.Reference == ref && m.AmountMinor == 50000 {
			found = true
		}
	}
	if !found {
		t.Fatalf("credit %q not in inbound list %+v", ref, list)
	}
}

func statementArithmetic(t *testing.T, h Harness) {
	ctx := context.Background()
	src := h.Bank.StatementSource()
	before, err := src.Statement(ctx, "ZAR", today())
	if err != nil {
		t.Fatalf("Statement: %v", err)
	}
	if err := h.Inbound("ZAR", 1000, newID("in"), false); err != nil {
		t.Fatalf("inject: %v", err)
	}
	if _, err := h.Bank.Rails().Pay(ctx, goodPay(newID("pay"))); err != nil {
		t.Fatalf("Pay: %v", err)
	}
	after, err := src.Statement(ctx, "ZAR", today())
	if err != nil {
		t.Fatalf("Statement: %v", err)
	}
	var in, out int64
	for _, m := range after.Movements {
		if m.Direction == "IN" {
			in += m.AmountMinor
		} else {
			out += m.AmountMinor
		}
	}
	if after.OpeningMinor != before.OpeningMinor {
		t.Fatalf("opening changed within the day: %d -> %d", before.OpeningMinor, after.OpeningMinor)
	}
	if after.ClosingMinor != after.OpeningMinor+in-out {
		t.Fatalf("closing %d != opening %d + in %d - out %d", after.ClosingMinor, after.OpeningMinor, in, out)
	}
}

func partnerSeparate(t *testing.T, h Harness) {
	ctx := context.Background()
	providers := h.Bank.ForeignCurrency()
	if len(providers) != 1 || providers[0].Asset() != "USD" {
		t.Fatalf("v1 must expose exactly one USD partner, got %d providers", len(providers))
	}
	p := providers[0]
	acc, err := p.OpenAccount(ctx, banklayer.OpenAccountRequest{Owner: newID("owner"), Asset: "USD"})
	if err != nil || acc.ID == "" || acc.Asset != "USD" {
		t.Fatalf("partner OpenAccount = %+v, %v", acc, err)
	}
	sponsorBefore, err := h.Bank.StatementSource().Statement(ctx, "USD", today())
	if err != nil {
		t.Fatalf("sponsor Statement: %v", err)
	}
	if err := h.Inbound("USD", 777, newID("usd-in"), true); err != nil {
		t.Fatalf("inject partner credit: %v", err)
	}
	partnerSt, err := p.Statement(ctx, today())
	if err != nil {
		t.Fatalf("partner Statement: %v", err)
	}
	if partnerSt.ClosingMinor < 777 {
		t.Fatalf("partner closing %d does not include the credit", partnerSt.ClosingMinor)
	}
	sponsorAfter, err := h.Bank.StatementSource().Statement(ctx, "USD", today())
	if err != nil {
		t.Fatalf("sponsor Statement: %v", err)
	}
	if sponsorAfter.ClosingMinor != sponsorBefore.ClosingMinor {
		t.Fatalf("a partner credit moved the sponsor pool: %d -> %d", sponsorBefore.ClosingMinor, sponsorAfter.ClosingMinor)
	}
}

func partnerPayout(t *testing.T, h Harness) {
	ctx := context.Background()
	p := h.Bank.ForeignCurrency()[0]
	if err := h.Inbound("USD", 5000, newID("usd-in"), true); err != nil {
		t.Fatalf("inject: %v", err)
	}
	before, err := p.Statement(ctx, today())
	if err != nil {
		t.Fatalf("partner Statement: %v", err)
	}
	req := banklayer.PayRequest{
		ID: newID("usd-pay"), Asset: "USD", AmountMinor: 1200, Beneficiary: "Cloud Vendor",
		AccountNumber: "9876543210", BankCode: "BIC-XYZ", Reference: "INV-USD-1",
	}
	if _, err := p.Payout(ctx, req); err != nil {
		t.Fatalf("Payout: %v", err)
	}
	var last banklayer.PayResult
	for i := 0; i < 3; i++ {
		last, err = p.PayoutStatus(ctx, req.ID)
		if err != nil {
			t.Fatalf("PayoutStatus: %v", err)
		}
	}
	if last.Status != banklayer.StatusSettled {
		t.Fatalf("final status = %q, want SETTLED", last.Status)
	}
	after, err := p.Statement(ctx, today())
	if err != nil {
		t.Fatalf("partner Statement: %v", err)
	}
	if after.ClosingMinor != before.ClosingMinor-req.AmountMinor {
		t.Fatalf("partner closing %d, want %d", after.ClosingMinor, before.ClosingMinor-req.AmountMinor)
	}
}

func dutiesMatchRoute(t *testing.T, h Harness) {
	ctx := context.Background()
	if h.Bank.PrudentialReporter().Required() {
		t.Fatal("prudential returns must not be required on the sponsor route")
	}
	if err := h.Bank.PrudentialReporter().Submit(ctx, "2026-10", []byte("{}")); !errors.Is(err, banklayer.ErrNotSupported) {
		t.Fatalf("Submit err = %v, want ErrNotSupported", err)
	}
	ins := h.Bank.DepositInsurance()
	if ins.Duty() != "sponsor" {
		t.Fatalf("insurance duty = %q, want sponsor", ins.Duty())
	}
	if err := ins.Report(ctx, today(), "ZAR", 0); err != nil {
		t.Fatalf("Report: %v", err)
	}
	if err := ins.Report(ctx, today(), "ZAR", -1); !errors.Is(err, banklayer.ErrInvalid) {
		t.Fatalf("negative insured amount err = %v, want ErrInvalid", err)
	}
}
