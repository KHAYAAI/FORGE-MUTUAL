package lending

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"connectrpc.com/connect"

	lendingv1 "github.com/khayaai/forge-mutual/pkg/api/forge/lending/v1"
)

// StatementHash identifies the exact pre-agreement text a customer was shown.
func StatementHash(statement string) string {
	h := sha256.Sum256([]byte(statement))
	return hex.EncodeToString(h[:])
}

func rand(minor string) string {
	n, _ := strconv.ParseInt(minor, 10, 64)
	return fmt.Sprintf("R%d.%02d", n/100, n%100)
}

func pct(bps uint32) string { return fmt.Sprintf("%d.%02d%%", bps/100, bps%100) }

// PreAgreement builds the pre-agreement statement and quotation (National Credit Act s.92) from the approved terms. It is
// generated, not typed, so what the customer is shown is what the facility says; the customer acknowledges it by its hash.
//
// DRAFT. The prescribed content and form must be confirmed by the lawyer before credit is offered (VERIFY, docs/QUESTIONS.md).
// It says plainly that Forge is not yet registered as a credit provider unless CreditProviderReg is set.
func (s *Service) PreAgreement(f *lendingv1.Facility) string {
	p, _ := s.product(f.ProductId)
	reg := strings.TrimSpace(s.CreditProviderReg)
	if reg == "" {
		reg = "NOT YET REGISTERED (credit cannot be offered until Forge Mutual holds a National Credit Regulator registration)"
	}
	var b strings.Builder
	b.WriteString("PRE-AGREEMENT STATEMENT AND QUOTATION\n\n")
	fmt.Fprintf(&b, "Credit provider: Forge Mutual. National Credit Regulator registration number: %s.\n", reg)
	fmt.Fprintf(&b, "Credit product: %s.\n", p.Name)
	fmt.Fprintf(&b, "Amount of credit: %s.\n", rand(f.ApprovedMinor))
	fmt.Fprintf(&b, "Interest rate: %s a year", pct(f.RateBps))
	if f.RateBps == 0 {
		b.WriteString(" (this product charges a fee, not interest)")
	}
	b.WriteString(". The rate is fixed unless we tell you otherwise before you accept (confirm with the lawyer).\n")
	if f.FeeBps > 0 {
		fmt.Fprintf(&b, "Fee: %s of the amount of credit.\n", pct(f.FeeBps))
	}
	if f.SweepBps > 0 {
		fmt.Fprintf(&b, "Repayment: %s of the money paid into your account is taken until the amount and fee are repaid.\n", pct(f.SweepBps))
	}
	if f.TermMonths > 0 {
		fmt.Fprintf(&b, "Term: %d months, in %d monthly instalments.\n", f.TermMonths, len(f.Schedule))
		if len(f.Schedule) > 0 {
			var first, second int64
			first, _ = strconv.ParseInt(f.Schedule[0].PrincipalMinor, 10, 64)
			second, _ = strconv.ParseInt(f.Schedule[0].InterestMinor, 10, 64)
			fmt.Fprintf(&b, "First instalment: %s, due on %s.\n", rand(strconv.FormatInt(first+second, 10)), f.Schedule[0].DueOn)
		}
	}
	fmt.Fprintf(&b, "Total you will pay if you pay on time: %s.\n", rand(f.TotalRepayableMinor))
	b.WriteString("\nIf you do not pay on time: we will send you a written notice before taking any further step. You may then refer the agreement to a debt counsellor, an alternative dispute resolution agent, the consumer court or an ombud, or agree a plan to bring your payments up to date. We may charge interest on what you owe until it is paid, but no other penalty fee.\n")
	b.WriteString("\nYour rights: you may repay early, in whole or in part. You may ask us for a statement at any time; we also send one each month. You may complain to us, then to the Ombudsman for Banking Services and the National Credit Regulator.\n")
	b.WriteString("\nBefore you accept, make sure you can afford the instalments. Credit you cannot afford can harm your credit record.\n")
	b.WriteString("\nThis statement is generated from the terms we approved. It is a draft form that has not yet been confirmed by our lawyer, and Forge Mutual is not yet offering credit to the public.")
	return b.String()
}

// GetPreAgreement returns the statement a customer must acknowledge before accepting an approved offer.
func (s *Service) GetPreAgreement(ctx context.Context, req *connect.Request[lendingv1.GetPreAgreementRequest]) (*connect.Response[lendingv1.GetPreAgreementResponse], error) {
	m := req.Msg
	if m.PartyId == "" || m.FacilityId == "" {
		return nil, invalid("party_id and facility_id are required")
	}
	f, err := s.facility(ctx, m.PartyId, m.FacilityId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no such offer"))
	}
	if f.Status != "APPROVED" {
		return nil, refused("there is no approved offer to show a statement for")
	}
	st := s.PreAgreement(f)
	return connect.NewResponse(&lendingv1.GetPreAgreementResponse{Statement: st, StatementHash: StatementHash(st)}), nil
}
