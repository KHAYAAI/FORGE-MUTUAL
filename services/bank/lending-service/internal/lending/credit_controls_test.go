package lending

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	lendingv1 "github.com/khayaai/forge-mutual/pkg/api/forge/lending/v1"
)

type consentMap map[string]bool

func (c consentMap) Allowed(_ context.Context, party, purpose, scope string) (bool, error) {
	return c[party+"|"+purpose+"|"+scope], nil
}

type downBureau struct{}

func (downBureau) Simulated() bool { return true }
func (downBureau) Check(context.Context, string) (BureauResult, error) {
	return BureauResult{}, errors.New("bureau away")
}

func (w *world) applyAs(t *testing.T, party string) (*lendingv1.Facility, error) {
	t.Helper()
	res, err := w.svc.ApplyForCredit(context.Background(), connect.NewRequest(&lendingv1.ApplyForCreditRequest{
		IdempotencyKey: "bureau-" + party, PartyId: party, ProductId: "term-loan", AmountMinor: "6000000", TermMonths: 6,
		CurrentAccountId: w.current, Purpose: "stock", Actor: "Thandi"}))
	if err != nil {
		return nil, err
	}
	return res.Msg.Facility, nil
}

// A credit enquiry needs consent, a bureau that cannot be reached means no assessment, and an adverse record needs a written reason.
func TestTheBureauIsAskedWithConsentFailsClosedAndAdverseNeedsAReason(t *testing.T) {
	w := setup(t)
	ctx := context.Background()
	w.svc.Owners = owners{"party-1": {w.current: "ZAR"}, "party-1-adverse": {w.current: "ZAR"}}
	consents := consentMap{}
	w.svc.Bureau, w.svc.Consents = SimBureau{}, consents

	if _, err := w.applyAs(t, "party-1"); connect.CodeOf(err) != connect.CodeFailedPrecondition || !strings.Contains(err.Error(), "consent") {
		t.Fatalf("no consent to a credit enquiry: %v", err)
	}
	consents["party-1|credit_bureau_check|enquiry"] = true
	consents["party-1-adverse|credit_bureau_check|enquiry"] = true
	f, err := w.applyAs(t, "party-1")
	if err != nil || f.Assessment.BureauReference == "" || f.Assessment.BureauScore < 450 || f.Assessment.BureauAdverse {
		t.Fatalf("assessment with a bureau: %+v %v", f.Assessment, err)
	}

	w.svc.Bureau = downBureau{}
	if _, err := w.svc.ApplyForCredit(ctx, connect.NewRequest(&lendingv1.ApplyForCreditRequest{IdempotencyKey: "other", PartyId: "party-1", ProductId: "term-loan",
		AmountMinor: "7000000", TermMonths: 6, CurrentAccountId: w.current, Actor: "Thandi"})); connect.CodeOf(err) != connect.CodeUnavailable {
		t.Fatalf("no answer from the bureau must refuse, not assess blind: %v", err)
	}
	var n int
	_ = w.svc.Pool.QueryRow(ctx, `select count(*) from facility where idempotency_key like '%other'`).Scan(&n)
	if n != 0 {
		t.Fatal("a refused application must leave nothing behind")
	}
	w.svc.Bureau = SimBureau{}

	adv, err := w.applyAs(t, "party-1-adverse")
	if err != nil || !adv.Assessment.BureauAdverse || adv.Assessment.BureauScore != 380 {
		t.Fatalf("adverse record: %+v %v", adv.Assessment, err)
	}
	decide := func(note string) error {
		_, err := w.svc.DecideCreditApplication(ctx, connect.NewRequest(&lendingv1.DecideCreditApplicationRequest{FacilityId: adv.FacilityId, Approve: true, Note: note, Officer: "Sipho"}))
		return err
	}
	if connect.CodeOf(decide("fine")) != connect.CodeInvalidArgument {
		t.Fatal("approving an adverse record needs a written reason")
	}
	if err := decide("Adverse listing is a settled 2021 judgment; inflows are strong and steady"); err != nil {
		t.Fatal(err)
	}
}

// The pre-agreement statement is generated from the approved terms and must be acknowledged, by its hash, before acceptance.
func TestThePreAgreementStatementMustBeAcknowledgedBeforeAcceptance(t *testing.T) {
	w := setup(t)
	ctx := context.Background()
	w.svc.RequirePreAgreement = true
	f := w.apply(t, "term-loan", 6_000_000, 6)
	get := func(party string) (*lendingv1.GetPreAgreementResponse, error) {
		r, err := w.svc.GetPreAgreement(ctx, connect.NewRequest(&lendingv1.GetPreAgreementRequest{PartyId: party, FacilityId: f.FacilityId}))
		if err != nil {
			return nil, err
		}
		return r.Msg, nil
	}
	if _, err := get("party-1"); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("no statement before approval: %v", err)
	}
	w.approve(t, f.FacilityId, "")
	if _, err := get("someone-else"); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("another party's statement: %v", err)
	}
	st, err := get("party-1")
	if err != nil {
		t.Fatal(err)
	}
	for _, need := range []string{"PRE-AGREEMENT STATEMENT AND QUOTATION", "NOT YET REGISTERED", "Amount of credit: R60000.00", "Interest rate: 14.50% a year",
		"Total you will pay if you pay on time", "debt counsellor", "Ombudsman for Banking Services", "not yet offering credit to the public"} {
		if !strings.Contains(st.Statement, need) {
			t.Errorf("the statement lacks %q:\n%s", need, st.Statement)
		}
	}
	accept := func(hash string) error {
		_, err := w.svc.AcceptOffer(ctx, connect.NewRequest(&lendingv1.AcceptOfferRequest{PartyId: "party-1", FacilityId: f.FacilityId, Actor: "Thandi", AcceptsTerms: true, PreAgreementHash: hash}))
		return err
	}
	if connect.CodeOf(accept("")) != connect.CodeFailedPrecondition || connect.CodeOf(accept("not-the-hash")) != connect.CodeFailedPrecondition {
		t.Fatal("acceptance without the current statement's hash must be refused")
	}
	if err := accept(st.StatementHash); err != nil {
		t.Fatal(err)
	}
	if again, _ := w.svc.facility(ctx, "party-1", f.FacilityId); again.Status != "ACTIVE" {
		t.Fatalf("status %s", again.Status)
	}
	w.svc.CreditProviderReg = "NCRCP9999"
	if s := w.svc.PreAgreement(&lendingv1.Facility{ProductId: "term-loan", ApprovedMinor: "100", RateBps: 1450, TotalRepayableMinor: "120"}); strings.Contains(s, "NOT YET REGISTERED") || !strings.Contains(s, "NCRCP9999") {
		t.Fatal("a registered provider's number replaces the warning")
	}
}

// A write-off needs a default notice, enough days past due, and two different people who are not the applicant. The debt moves off the
// loan account and into WRITE_OFF exactly once.
func TestWriteOffNeedsANoticeTimeAndTwoPeopleAndHappensOnce(t *testing.T) {
	w := setup(t)
	ctx := context.Background()
	f := w.apply(t, "term-loan", 6_000_000, 6)
	w.approve(t, f.FacilityId, "")
	f = w.accept(t, f.FacilityId)
	loan := w.loanAccount(t, f.FacilityId)

	w.at(time.Date(2026, 10, 15, 6, 0, 0, 0, time.UTC))
	_, _, _ = w.svc.Collect(ctx, *w.clock)
	w.ledger.Seed(w.current, -w.ledger.Balance(w.current))
	w.at(time.Date(2026, 11, 15, 6, 0, 0, 0, time.UTC))
	_, _, _ = w.svc.Collect(ctx, *w.clock)

	writeOff := func(officer, approver string) error {
		_, err := w.svc.WriteOffFacility(ctx, connect.NewRequest(&lendingv1.WriteOffFacilityRequest{FacilityId: f.FacilityId, Officer: officer, Approver: approver, Reason: "no contact for 90 days; legal route started"}))
		return err
	}
	if connect.CodeOf(writeOff("Sipho", "SIPHO")) != connect.CodePermissionDenied {
		t.Fatal("one person cannot be both")
	}
	if connect.CodeOf(writeOff("Thandi", "Lerato")) != connect.CodePermissionDenied {
		t.Fatal("the applicant cannot take part")
	}
	w.at(time.Date(2026, 12, 6, 6, 0, 0, 0, time.UTC)) // 21 days past due
	_, _, _ = w.svc.Collect(ctx, *w.clock)
	_, _ = w.svc.IssueNotices(ctx)
	if connect.CodeOf(writeOff("Sipho", "Lerato")) != connect.CodeFailedPrecondition {
		t.Fatal("21 days past due is not enough")
	}
	w.at(time.Date(2027, 2, 20, 6, 0, 0, 0, time.UTC)) // 97 days past due
	_, _, _ = w.svc.Collect(ctx, *w.clock)
	if _, err := w.svc.Pool.Exec(ctx, `delete from default_notice where facility_id = $1`, f.FacilityId); err != nil {
		t.Fatal(err)
	}
	if err := writeOff("Sipho", "Lerato"); connect.CodeOf(err) != connect.CodeFailedPrecondition || !strings.Contains(err.Error(), "default notice") {
		t.Fatalf("nothing is written off that the customer was never warned about: %v", err)
	}
	_, _ = w.svc.IssueNotices(ctx)
	owedBefore, _ := w.svc.balance(ctx, loan)
	if owedBefore >= 0 {
		t.Fatalf("something should be owed: %d", owedBefore)
	}
	if err := writeOff("Sipho", "Lerato"); err != nil {
		t.Fatal(err)
	}
	got, _ := w.svc.facility(ctx, "", f.FacilityId)
	if got.Status != "WRITTEN_OFF" || got.WrittenOffMinor == "0" || w.ledger.Balance(loan) != 0 || w.ledger.Balance(w.svc.WriteOff) != owedBefore {
		t.Fatalf("written off: %s %s loan balance %d write-off account %d (owed %d)", got.Status, got.WrittenOffMinor, w.ledger.Balance(loan), w.ledger.Balance(w.svc.WriteOff), owedBefore)
	}
	before := w.ledger.Balance(w.svc.WriteOff)
	if err := writeOff("Sipho", "Lerato"); err != nil || w.ledger.Balance(w.svc.WriteOff) != before {
		t.Fatalf("doing it again must change nothing: %v", err)
	}
	if paid, missed, _ := w.svc.Collect(ctx, *w.clock); paid != 0 || missed != 0 {
		t.Fatalf("a written-off facility is no longer collected: %d %d", paid, missed)
	}
	res, err := w.svc.GetStatement(ctx, connect.NewRequest(&lendingv1.GetStatementRequest{PartyId: "party-1", FacilityId: f.FacilityId, Period: "2027-02"}))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range res.Msg.Entries {
		found = found || e.Kind == "WRITTEN_OFF"
	}
	if !found {
		t.Fatal("the write-off must appear on the statement")
	}
}

func (w *world) loanAccount(t *testing.T, facility string) string {
	t.Helper()
	var id string
	if err := w.svc.Pool.QueryRow(context.Background(), `select loan_account_id from facility where facility_id = $1`, facility).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}
