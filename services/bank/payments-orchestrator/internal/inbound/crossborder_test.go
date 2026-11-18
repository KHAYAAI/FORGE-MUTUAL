package inbound

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"

	paymentsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/payments/v1"
)

func classify(c *CrossBorder, party, credit, code, note string) (*paymentsv1.CrossBorderCredit, error) {
	res, err := c.ClassifyCrossBorderCredit(context.Background(), connect.NewRequest(&paymentsv1.ClassifyCrossBorderCreditRequest{
		PartyId: party, CreditId: credit, ReasonCode: code, Note: note, Actor: "Naledi",
	}))
	if err != nil {
		return nil, err
	}
	return res.Msg.Credit, nil
}

// A euro and a pound credit book to their own customers from their own partner mirrors, and each one is recorded
// for cross-border reporting, awaiting its reason. A rand credit is not cross-border and is not recorded.
func TestForeignCreditsAreRecordedForCrossBorderReporting(t *testing.T) {
	ctx := context.Background()
	w, fl, bank, pool := setup(t)
	for _, cur := range []string{"EUR", "GBP"} {
		if err := w.RegisterReference(ctx, "FORGE-"+cur+"-1", cur, cur+"-acc-1", "party-x"); err != nil {
			t.Fatal(err)
		}
		if _, err := bank.InjectInbound(cur, 12_345, "FORGE-"+cur+"-1", true); err != nil {
			t.Fatal(err)
		}
		if n, err := w.Ingest(ctx, cur, time.Hour); err != nil || n != 1 {
			t.Fatalf("%s ingest: %d, %v", cur, n, err)
		}
	}
	_ = w.RegisterReference(ctx, "FORGE-ZAR-1", "ZAR", "zar-acc-1", "party-x")
	_, _ = bank.InjectInbound("ZAR", 500, "FORGE-ZAR-1", false)
	if _, err := w.Ingest(ctx, "ZAR", time.Hour); err != nil {
		t.Fatal(err)
	}
	for _, legs := range fl.transfers {
		cur := legs[0].Amount.Asset
		if cur != "ZAR" && legs[0].DebitAccountId != fl.account("system:pool:"+cur+":v1") {
			t.Fatalf("%s credit debited %s, want its own mirror", cur, legs[0].DebitAccountId)
		}
	}

	c := &CrossBorder{Pool: pool}
	res, err := c.ListCrossBorderCredits(ctx, connect.NewRequest(&paymentsv1.ListCrossBorderCreditsRequest{PartyId: "party-x"}))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Msg.Credits) != 2 {
		t.Fatalf("cross-border credits = %d, want 2 (EUR and GBP, not ZAR)", len(res.Msg.Credits))
	}
	for _, cr := range res.Msg.Credits {
		if cr.Status != "AWAITING_REASON" || cr.AmountMinor != "12345" || cr.ReasonCode != "" {
			t.Fatalf("credit = %+v", cr)
		}
	}
	var unrecorded int
	_ = pool.QueryRow(ctx, `select count(*) from inbound_credit ic where status = 'BOOKED' and asset <> 'ZAR'
		and not exists (select 1 from cross_border_credit c where c.asset = ic.asset and c.movement_id = ic.movement_id)`).Scan(&unrecorded)
	if unrecorded != 0 {
		t.Fatalf("%d booked foreign credits have no cross-border record", unrecorded)
	}
}

// The customer gives a reason once. Another party cannot see or classify the credit; an unknown reason, a bare "other"
// and a second, different reason are refused; sending the same reason again is harmless.
func TestAReasonIsGivenOnceByTheOwner(t *testing.T) {
	ctx := context.Background()
	w, _, bank, pool := setup(t)
	_ = w.RegisterReference(ctx, "FORGE-USD-9", "USD", "usd-acc-9", "party-owner")
	m, _ := bank.InjectInbound("USD", 99_000, "FORGE-USD-9", true)
	if _, err := w.Ingest(ctx, "USD", time.Hour); err != nil {
		t.Fatal(err)
	}
	c := &CrossBorder{Pool: pool}
	id := "USD:" + m.ID

	if _, err := classify(c, "party-other", id, "services_software", ""); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("another party: %v", err)
	}
	if _, err := classify(c, "party-owner", id, "made_up", ""); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("unknown reason: %v", err)
	}
	if _, err := classify(c, "party-owner", id, "other", " "); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("other with no note: %v", err)
	}
	cr, err := classify(c, "party-owner", id, "services_software", "Invoice INV-2026-118")
	if err != nil || cr.Status != "CLASSIFIED" || cr.ReasonCode != "services_software" || cr.ClassifiedBy != "Naledi" || cr.ClassifiedAt == "" {
		t.Fatalf("classify: %+v, %v", cr, err)
	}
	if again, err := classify(c, "party-owner", id, "services_software", "Invoice INV-2026-118"); err != nil || again.ClassifiedAt != cr.ClassifiedAt {
		t.Fatalf("the same reason again: %+v, %v", again, err)
	}
	if _, err := classify(c, "party-owner", id, "loan_received", ""); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("a changed reason: %v", err)
	}
}

// A reference is registered once; the same binding again is harmless; moving it is refused; a party lists only its own.
func TestReferencesAreRegisteredOnceAndListedForTheirOwner(t *testing.T) {
	ctx := context.Background()
	_, _, _, pool := setup(t)
	r := &References{Pool: pool}
	reg := func(ref, acc, party string) error {
		_, err := r.RegisterInboundReference(ctx, connect.NewRequest(&paymentsv1.RegisterInboundReferenceRequest{
			Reference: ref, Asset: "EUR", AccountId: acc, PartyId: party,
		}))
		return err
	}
	if err := reg("FORGEEURPB1", "eur-1", "party-a"); err != nil {
		t.Fatal(err)
	}
	if err := reg("FORGEEURPB1", "eur-1", "party-a"); err != nil {
		t.Fatalf("the same binding again: %v", err)
	}
	if err := reg("FORGEEURPB1", "eur-2", "party-b"); connect.CodeOf(err) != connect.CodeAlreadyExists {
		t.Fatalf("moving a reference: %v", err)
	}
	if err := reg(" padded", "eur-3", "party-a"); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("a padded reference: %v", err)
	}
	res, err := r.ListInboundReferences(ctx, connect.NewRequest(&paymentsv1.ListInboundReferencesRequest{PartyId: "party-b"}))
	if err != nil || len(res.Msg.References) != 0 {
		t.Fatalf("party-b sees %v, %v", res.Msg.GetReferences(), err)
	}
}
