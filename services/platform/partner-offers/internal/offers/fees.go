package offers

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	ledgerv1 "github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1"
	offersv1 "github.com/khayaai/forge-mutual/pkg/api/forge/offers/v1"
	"github.com/khayaai/forge-mutual/pkg/systemaccounts"
)

// Referral fees. A provider may pay Forge for introducing a customer, but only up to the cap on the offer, which is the same number the
// customer was told ("Forge Mutual may receive a referral fee of up to R..."). A fee is booked only when the provider's payment has arrived: the
// ledger entry debits the sponsor pool (where the money landed) and credits fee income, and carries the bank statement reference, so reconciliation
// can match it to the receipt and a receipt can never be booked twice. Two people must be involved, and the total for a referral never exceeds the cap.

const codeFee = 30 // FEE

var bankRef = regexp.MustCompile(`^[A-Za-z0-9._/-]{3,64}$`)

// EnsureFeeAccounts creates (or finds) the accounts a fee moves between.
func (s *Service) EnsureFeeAccounts(ctx context.Context) error {
	if s.Ledger == nil {
		return nil
	}
	var err error
	if s.SponsorPool, err = systemaccounts.Ensure(ctx, s.Ledger, "BANK", "ZAR", 3003, "system:pool:ZAR:v1"); err != nil {
		return err
	}
	s.FeeIncome, err = systemaccounts.Ensure(ctx, s.Ledger, "BANK", "ZAR", 2002, "system:referral-fees:ZAR:v1")
	return err
}

// RecordReferralFee books one received referral fee.
func (s *Service) RecordReferralFee(ctx context.Context, r *connect.Request[offersv1.RecordReferralFeeRequest]) (*connect.Response[offersv1.RecordReferralFeeResponse], error) {
	if err := s.allowed(); err != nil {
		return nil, err
	}
	m := r.Msg
	officer := strings.TrimSpace(m.Officer)
	amount, err := strconv.ParseInt(strings.TrimSpace(m.AmountMinor), 10, 64)
	if m.ReferralId == "" || officer == "" || err != nil || amount <= 0 || !bankRef.MatchString(m.BankReference) {
		return nil, invalid("referral_id, officer, a positive amount in cents and a bank reference of 3 to 64 letters, digits or . _ / - are required")
	}
	if s.Ledger == nil || s.SponsorPool == "" || s.FeeIncome == "" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("no ledger is connected, so no fee can be booked"))
	}
	var out *offersv1.ReferralFee
	awaiting := false
	err = pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		// One referral at a time, so two receipts cannot together pass the cap.
		var offerID string
		if err := tx.QueryRow(ctx, `select offer_id from referral where referral_id = $1 for update`, m.ReferralId).Scan(&offerID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return connect.NewError(connect.CodeNotFound, errors.New("no such referral"))
			}
			return err
		}
		var ref string
		var amt int64
		var by, appr string
		var at time.Time
		err := tx.QueryRow(ctx, `select referral_id, amount_minor, recorded_by, approved_by, recorded_at from referral_fee where bank_reference = $1`, m.BankReference).Scan(&ref, &amt, &by, &appr, &at)
		if err == nil {
			if ref == m.ReferralId && amt == amount {
				out = &offersv1.ReferralFee{ReferralId: ref, AmountMinor: strconv.FormatInt(amt, 10), BankReference: m.BankReference, RecordedBy: by, ApprovedBy: appr, RecordedAt: at.UTC().Format(time.RFC3339)}
				return nil
			}
			return connect.NewError(connect.CodeAlreadyExists, errors.New("that bank reference has already been booked"))
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		var cap, booked int64
		if err := tx.QueryRow(ctx, `select fee_cap_minor from offer where offer_id = $1`, offerID).Scan(&cap); err != nil {
			return err
		}
		if cap <= 0 {
			return connect.NewError(connect.CodeFailedPrecondition, errors.New("this offer pays Forge nothing, so no fee can be booked against it"))
		}
		if err := tx.QueryRow(ctx, `select coalesce(sum(amount_minor),0) from referral_fee where referral_id = $1`, m.ReferralId).Scan(&booked); err != nil {
			return err
		}
		if booked+amount > cap {
			return connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("that would take the fees for this referral to %d, over the %d the customer was told Forge may receive", booked+amount, cap))
		}
		// Two people: the first proposes, a different one books. The proposal books nothing.
		var pRef, pBy string
		var pAmt int64
		err = tx.QueryRow(ctx, `select referral_id, amount_minor, proposed_by from referral_fee_proposal where bank_reference = $1 for update`, m.BankReference).Scan(&pRef, &pAmt, &pBy)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			if _, err := tx.Exec(ctx, `insert into referral_fee_proposal (bank_reference, referral_id, amount_minor, proposed_by, proposed_at) values ($1,$2,$3,$4,$5)`,
				m.BankReference, m.ReferralId, amount, officer, s.now()); err != nil {
				return err
			}
			awaiting = true
			return nil
		case err != nil:
			return err
		case pRef != m.ReferralId || pAmt != amount:
			return connect.NewError(connect.CodeFailedPrecondition, errors.New("a different fee is already waiting for confirmation under that bank reference"))
		case strings.EqualFold(pBy, officer):
			awaiting = true // the same person again: still waiting for someone else
			return nil
		}
		key := "referral-fee:" + m.BankReference
		res, err := s.Ledger.CreateTransfers(ctx, connect.NewRequest(&ledgerv1.CreateTransfersRequest{IdempotencyKey: key, BusinessTxnId: key, EntityCode: "BANK",
			Legs:  []*ledgerv1.TransferLeg{{DebitAccountId: s.SponsorPool, CreditAccountId: s.FeeIncome, Code: codeFee, Amount: &ledgerv1.Money{Asset: "ZAR", AmountMinor: strconv.FormatInt(amount, 10)}}},
			Actor: officer, Reason: "referral fee received, bank reference " + m.BankReference}))
		if err != nil {
			return connect.NewError(connect.CodeUnavailable, err)
		}
		if res.Msg.Status != ledgerv1.Status_STATUS_OK && res.Msg.Status != ledgerv1.Status_STATUS_REPLAYED {
			return connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("the ledger refused it: %v", res.Msg.RejectionCodes))
		}
		now := s.now()
		if _, err := tx.Exec(ctx, `insert into referral_fee (bank_reference, referral_id, amount_minor, recorded_by, approved_by, recorded_at, ledger_key) values ($1,$2,$3,$4,$5,$6,$7)`,
			m.BankReference, m.ReferralId, amount, pBy, officer, now, key); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `delete from referral_fee_proposal where bank_reference = $1`, m.BankReference); err != nil {
			return err
		}
		out = &offersv1.ReferralFee{ReferralId: m.ReferralId, AmountMinor: strconv.FormatInt(amount, 10), BankReference: m.BankReference, RecordedBy: pBy, ApprovedBy: officer, RecordedAt: now.Format(time.RFC3339)}
		return nil
	})
	if err != nil {
		return nil, wrap(err)
	}
	return connect.NewResponse(&offersv1.RecordReferralFeeResponse{Fee: out, AwaitingSecondPerson: awaiting}), nil
}

// ListAdminReferrals shows compliance and finance every referral, whether the customer's details were sent, and the fees booked.
func (s *Service) ListAdminReferrals(ctx context.Context, _ *connect.Request[offersv1.ListAdminReferralsRequest]) (*connect.Response[offersv1.ListAdminReferralsResponse], error) {
	if err := s.allowed(); err != nil {
		return nil, err
	}
	rows, err := s.Pool.Query(ctx, `select r.referral_id, r.offer_id, o.title, p.provider_id, p.name, r.party_id, r.created_at,
		coalesce(d.status,'NOT_CONFIGURED'), coalesce(d.detail,''), d.sent_at, o.fee_cap_minor,
		coalesce((select sum(amount_minor) from referral_fee f where f.referral_id = r.referral_id),0)
		from referral r join offer o using (offer_id) join provider p using (provider_id) left join referral_delivery d using (referral_id)
		order by r.created_at desc limit 500`)
	if err != nil {
		return nil, wrap(err)
	}
	defer rows.Close()
	out := &offersv1.ListAdminReferralsResponse{}
	for rows.Next() {
		var a offersv1.AdminReferral
		var created time.Time
		var sentAt *time.Time
		var cap, booked int64
		if err := rows.Scan(&a.ReferralId, &a.OfferId, &a.OfferTitle, &a.ProviderId, &a.ProviderName, &a.PartyId, &created, &a.DeliveryStatus, &a.DeliveryDetail, &sentAt, &cap, &booked); err != nil {
			return nil, wrap(err)
		}
		a.CreatedAt, a.DeliveredAt = created.UTC().Format(time.RFC3339), tsPtr(sentAt)
		a.FeeCapMinor, a.FeesRecordedMinor = strconv.FormatInt(cap, 10), strconv.FormatInt(booked, 10)
		out.Referrals = append(out.Referrals, &a)
	}
	return connect.NewResponse(out), wrap(rows.Err())
}
