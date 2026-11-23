// Package lending runs credit facilities: overdrafts, working-capital term loans and revenue-based financing.
package lending

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	consolidatedv1 "github.com/khayaai/forge-mutual/pkg/api/forge/consolidated/v1"
	ledgerv1 "github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1/ledgerv1connect"
	lendingv1 "github.com/khayaai/forge-mutual/pkg/api/forge/lending/v1"
	"github.com/khayaai/forge-mutual/pkg/licence"
	"github.com/khayaai/forge-mutual/pkg/systemaccounts"
	"github.com/khayaai/forge-mutual/pkg/uuid"
	"github.com/khayaai/forge-mutual/services/bank/lending-service/internal/credit"
)

//go:embed schema.sql
var schema string

// Open applies the schema.
func Open(ctx context.Context, pool *pgxpool.Pool) error {
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `select pg_advisory_xact_lock(7753)`); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, schema)
		return err
	})
}

// Product is one credit product. Rates, fees, limits and the affordability rule are placeholders for the credit
// policy owner (docs/THRESHOLDS.md).
type Product struct {
	ID            string
	Name          string
	RateBps       uint32
	FeeBps        uint32
	SweepBps      uint32
	Min, Max      int64
	MaxTermMonths int
	Licence       licence.Product
}

// DefaultProducts are the products offered, with placeholder terms.
func DefaultProducts() []Product {
	return []Product{
		{ID: "overdraft", Name: "Overdraft facility", RateBps: 1675, Min: 500_000, Max: 50_000_000, Licence: licence.Overdraft},
		{ID: "term-loan", Name: "Working-capital loan", RateBps: 1450, Min: 1_000_000, Max: 500_000_000, MaxTermMonths: 60, Licence: licence.TermLoans},
		{ID: "revenue-based", Name: "Revenue-based financing", FeeBps: 800, SweepBps: 1000, Min: 1_000_000, Max: 200_000_000, Licence: licence.RevenueBased},
	}
}

// Accounts reports a party's accounts and their assets; Activity pages through an account's movements.
type (
	Ownership interface {
		Accounts(ctx context.Context, party string) (map[string]string, error)
	}
	Activity interface {
		GetActivity(context.Context, *connect.Request[consolidatedv1.GetActivityRequest]) (*connect.Response[consolidatedv1.GetActivityResponse], error)
	}
)

// Ledger codes.
const (
	codeDisbursement   = 66
	codeRepayment      = 67
	codeInterestCharge = 68
	namespace          = "4f2a9c7e-1d3b-5e60-8a14-7b9e0c2d5f31"
)

// InflowCodes are the transfer codes that count as money received from outside: a payment in, a card settlement and
// an invoice payment. Internal moves and loan payouts do not count.
var InflowCodes = map[uint32]bool{10: true, 52: true, 62: true}

// Service implements both handlers and runs the daily work.
type Service struct {
	Pool     *pgxpool.Pool
	Ledger   ledgerv1connect.LedgerServiceClient
	Owners   Ownership
	Activity Activity
	Products []Product
	Licence  licence.Gate
	Entity   string
	Income   string // INTEREST_INCOME, set by EnsureSystemAccounts
	Now      func() time.Time
}

// EnsureSystemAccounts creates (or finds) the INTEREST_INCOME account.
func (s *Service) EnsureSystemAccounts(ctx context.Context) error {
	id, err := systemaccounts.Ensure(ctx, s.Ledger, s.Entity, "ZAR", 2008, "system:interest-income:ZAR:v1")
	s.Income = id
	return err
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s *Service) product(id string) (Product, bool) {
	for _, p := range s.Products {
		if p.ID == id {
			return p, true
		}
	}
	return Product{}, false
}

func invalid(msg string) error { return connect.NewError(connect.CodeInvalidArgument, errors.New(msg)) }
func refused(msg string) error {
	return connect.NewError(connect.CodeFailedPrecondition, errors.New(msg))
}

func wrap(err error) error {
	var ce *connect.Error
	if err == nil || errors.As(err, &ce) {
		return err
	}
	return connect.NewError(connect.CodeInternal, fmt.Errorf("lending: %w", err))
}

func amountOf(s string) (int64, error) {
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil || v <= 0 {
		return 0, invalid("amount_minor must be a positive integer")
	}
	return v, nil
}

func hash(parts ...string) string {
	h := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(h[:])
}

// ListLoanProducts implements the handler.
func (s *Service) ListLoanProducts(context.Context, *connect.Request[lendingv1.ListLoanProductsRequest]) (*connect.Response[lendingv1.ListLoanProductsResponse], error) {
	out := &lendingv1.ListLoanProductsResponse{}
	for _, p := range s.Products {
		out.Products = append(out.Products, &lendingv1.LoanProduct{
			ProductId: p.ID, Name: p.Name, RateBps: p.RateBps, FeeBps: p.FeeBps, SweepBps: p.SweepBps,
			MinMinor: fmt.Sprint(p.Min), MaxMinor: fmt.Sprint(p.Max), MaxTermMonths: uint32(p.MaxTermMonths),
			Available: s.Licence.Allow(p.Licence) == nil,
		})
	}
	return connect.NewResponse(out), nil
}

// inflow90d sums the money received into an account in the last 90 days.
func (s *Service) inflow90d(ctx context.Context, party, account string) (int64, uint64, error) {
	since := s.now().AddDate(0, 0, -90)
	var total int64
	var after uint64
	for page := 0; page < 100; page++ {
		res, err := s.Activity.GetActivity(ctx, connect.NewRequest(&consolidatedv1.GetActivityRequest{PartyId: party, AccountId: account, AfterSeq: after, Limit: 500}))
		if err != nil {
			return 0, 0, err
		}
		for _, e := range res.Msg.Entries {
			at, _ := time.Parse(time.RFC3339, e.At)
			if e.Direction == "IN" && InflowCodes[e.Code] && !at.Before(since) {
				v, _ := strconv.ParseInt(e.AmountMinor, 10, 64)
				total += v
			}
		}
		if len(res.Msg.Entries) == 0 || res.Msg.NextAfterSeq == after {
			break
		}
		after = res.Msg.NextAfterSeq
	}
	return total, after, nil
}

// ApplyForCredit implements the handler. The application is assessed at once on the account's own history.
func (s *Service) ApplyForCredit(ctx context.Context, req *connect.Request[lendingv1.ApplyForCreditRequest]) (*connect.Response[lendingv1.ApplyForCreditResponse], error) {
	m := req.Msg
	p, ok := s.product(m.ProductId)
	if m.IdempotencyKey == "" || m.PartyId == "" || m.Actor == "" || !ok {
		return nil, invalid("idempotency_key, party_id, actor and a listed product_id are required")
	}
	if err := s.Licence.Allow(p.Licence); err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	amount, err := amountOf(m.AmountMinor)
	if err != nil {
		return nil, err
	}
	if amount < p.Min || amount > p.Max {
		return nil, invalid(fmt.Sprintf("the amount must be between %d and %d minor units", p.Min, p.Max))
	}
	term := int(m.TermMonths)
	if p.ID == "term-loan" && (term < 1 || term > p.MaxTermMonths) {
		return nil, invalid(fmt.Sprintf("term_months must be 1 to %d", p.MaxTermMonths))
	}
	if p.ID != "term-loan" {
		term = 0
	}
	if len(m.Purpose) > 200 {
		return nil, invalid("purpose must be at most 200 characters")
	}
	owned, err := s.Owners.Accounts(ctx, m.PartyId)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	if owned[m.CurrentAccountId] != "ZAR" {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("credit is paid to and repaid from the party's own rand account"))
	}
	key := "lending:" + m.PartyId + ":" + m.IdempotencyKey
	id, err := uuid.NewV5(namespace, key)
	if err != nil {
		return nil, wrap(err)
	}
	if f, err := s.facility(ctx, m.PartyId, id); err == nil {
		return connect.NewResponse(&lendingv1.ApplyForCreditResponse{Facility: f}), nil
	}
	inflow, _, err := s.inflow90d(ctx, m.PartyId, m.CurrentAccountId)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	a := &lendingv1.Assessment{
		Inflow_90DMinor: fmt.Sprint(inflow), MonthlyInflowMinor: fmt.Sprint(inflow / 3),
		SuggestedMaxMinor: fmt.Sprint(credit.SuggestedMax(p.ID, inflow)),
	}
	if inflow == 0 {
		a.Notes = append(a.Notes, "No money received into the account in the last 90 days.")
	}
	if amount > credit.SuggestedMax(p.ID, inflow) {
		a.Notes = append(a.Notes, "The amount is more than the affordability rule suggests.")
	}
	raw, _ := json.Marshal(a)
	if _, err := s.Pool.Exec(ctx, `insert into facility (facility_id, party_id, product_id, status, requested_minor, rate_bps, term_months,
		fee_bps, sweep_bps, current_account_id, purpose, applied_by, applied_at, idempotency_key, assessment)
		values ($1, $2, $3, 'ASSESSED', $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14) on conflict (idempotency_key) do nothing`,
		id, m.PartyId, p.ID, amount, p.RateBps, term, p.FeeBps, p.SweepBps, m.CurrentAccountId, strings.TrimSpace(m.Purpose),
		m.Actor, s.now(), key, raw); err != nil {
		return nil, wrap(err)
	}
	f, err := s.facility(ctx, m.PartyId, id)
	if err != nil {
		return nil, wrap(err)
	}
	return connect.NewResponse(&lendingv1.ApplyForCreditResponse{Facility: f}), nil
}

// facility reads one facility the party owns (party "" for staff), with its balance and schedule.
func (s *Service) facility(ctx context.Context, party, id string) (*lendingv1.Facility, error) {
	list, err := s.facilities(ctx, `where facility_id = $1 and ($2 = '' or party_id = $2)`, id, party)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no such facility"))
	}
	return list[0], nil
}

func (s *Service) facilities(ctx context.Context, where string, args ...any) ([]*lendingv1.Facility, error) {
	rows, err := s.Pool.Query(ctx, `select facility_id, party_id, product_id, status, requested_minor, approved_minor, rate_bps,
		term_months, fee_bps, sweep_bps, current_account_id, loan_account_id, purpose, applied_at, decided_at, decision_note,
		assessment, fee_owed_minor, days_past_due from facility `+where+` order by applied_at desc limit 200`, args...)
	if err != nil {
		return nil, err
	}
	type row struct {
		f        *lendingv1.Facility
		loan     string
		approved int64
		feeOwed  int64
	}
	var all []row
	for rows.Next() {
		var (
			f                          lendingv1.Facility
			req, approved, feeOwed     int64
			rate, term, fee, sweep, dp int32
			loan                       string
			applied                    time.Time
			decided                    *time.Time
			raw                        []byte
		)
		if err := rows.Scan(&f.FacilityId, &f.PartyId, &f.ProductId, &f.Status, &req, &approved, &rate, &term, &fee, &sweep,
			&f.CurrentAccountId, &loan, &f.Purpose, &applied, &decided, &f.DecisionNote, &raw, &feeOwed, &dp); err != nil {
			rows.Close()
			return nil, err
		}
		f.RequestedMinor, f.ApprovedMinor = fmt.Sprint(req), fmt.Sprint(approved)
		f.RateBps, f.TermMonths, f.FeeBps, f.SweepBps, f.DaysPastDue = uint32(rate), uint32(term), uint32(fee), uint32(sweep), uint32(dp)
		f.AppliedAt = applied.UTC().Format(time.RFC3339)
		if decided != nil {
			f.DecidedAt = decided.UTC().Format(time.RFC3339)
		}
		f.Assessment = &lendingv1.Assessment{}
		_ = json.Unmarshal(raw, f.Assessment)
		all = append(all, row{&f, loan, approved, feeOwed})
	}
	rows.Close()
	var out []*lendingv1.Facility
	for _, r := range all {
		f := r.f
		owed := int64(0)
		if r.loan != "" {
			bal, err := s.balance(ctx, r.loan)
			if err != nil {
				return nil, err
			}
			owed = -bal
		}
		f.OutstandingMinor = fmt.Sprint(owed + r.feeOwed)
		if f.ProductId == "overdraft" && f.Status == "ACTIVE" {
			f.AvailableMinor = fmt.Sprint(max(r.approved-owed, 0))
		} else {
			f.AvailableMinor = "0"
		}
		sched, err := s.schedule(ctx, f.FacilityId)
		if err != nil {
			return nil, err
		}
		f.Schedule = sched
		f.TotalRepayableMinor = fmt.Sprint(s.totalRepayable(f, r.approved, sched))
		if f.DefaultNotice, err = s.openNotice(ctx, f.FacilityId); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, nil
}

// totalRepayable is the quotation: for a loan the sum of its instalments, for revenue-based the advance plus the fee.
// An overdraft's cost depends on use; it shows the limit.
func (s *Service) totalRepayable(f *lendingv1.Facility, approved int64, sched []*lendingv1.Instalment) int64 {
	amount := approved
	if amount == 0 {
		amount, _ = strconv.ParseInt(f.RequestedMinor, 10, 64)
	}
	switch f.ProductId {
	case "term-loan":
		if len(sched) > 0 {
			var t int64
			for _, i := range sched {
				p, _ := strconv.ParseInt(i.PrincipalMinor, 10, 64)
				n, _ := strconv.ParseInt(i.InterestMinor, 10, 64)
				t += p + n
			}
			return t
		}
		s, err := credit.Schedule(amount, f.RateBps, int(f.TermMonths))
		if err != nil {
			return 0
		}
		return credit.TotalRepayable(s)
	case "revenue-based":
		return amount + credit.Fee(amount, f.FeeBps)
	}
	return amount
}

func (s *Service) schedule(ctx context.Context, facility string) ([]*lendingv1.Instalment, error) {
	rows, err := s.Pool.Query(ctx, `select number, due_on, principal_minor, interest_minor, status from instalment
		where facility_id = $1 order by number`, facility)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*lendingv1.Instalment
	for rows.Next() {
		var i lendingv1.Instalment
		var n int32
		var due time.Time
		var p, in int64
		if err := rows.Scan(&n, &due, &p, &in, &i.Status); err != nil {
			return nil, err
		}
		i.Number, i.DueOn, i.PrincipalMinor, i.InterestMinor = uint32(n), due.Format("2006-01-02"), fmt.Sprint(p), fmt.Sprint(in)
		out = append(out, &i)
	}
	return out, rows.Err()
}

func (s *Service) balance(ctx context.Context, account string) (int64, error) {
	res, err := s.Ledger.GetBalances(ctx, connect.NewRequest(&ledgerv1.GetBalancesRequest{EntityCode: s.Entity, AccountIds: []string{account}}))
	if err != nil {
		return 0, err
	}
	if len(res.Msg.Balances) != 1 {
		return 0, fmt.Errorf("balance of %s: not found", account)
	}
	return strconv.ParseInt(res.Msg.Balances[0].AvailableMinor, 10, 64)
}

// ListFacilities implements the handler. The decision note is for staff and is not shown to the customer.
func (s *Service) ListFacilities(ctx context.Context, req *connect.Request[lendingv1.ListFacilitiesRequest]) (*connect.Response[lendingv1.ListFacilitiesResponse], error) {
	if req.Msg.PartyId == "" {
		return nil, invalid("party_id is required")
	}
	list, err := s.facilities(ctx, `where party_id = $1`, req.Msg.PartyId)
	if err != nil {
		return nil, wrap(err)
	}
	for _, f := range list {
		f.DecisionNote, f.PartyId = "", ""
	}
	return connect.NewResponse(&lendingv1.ListFacilitiesResponse{Facilities: list}), nil
}

// ListCreditApplications implements the credit desk handler.
func (s *Service) ListCreditApplications(ctx context.Context, req *connect.Request[lendingv1.ListCreditApplicationsRequest]) (*connect.Response[lendingv1.ListCreditApplicationsResponse], error) {
	status := req.Msg.Status
	if status == "" {
		status = "ASSESSED"
	}
	list, err := s.facilities(ctx, `where status = $1`, status)
	if err != nil {
		return nil, wrap(err)
	}
	return connect.NewResponse(&lendingv1.ListCreditApplicationsResponse{Facilities: list}), nil
}

// DecideCreditApplication implements the credit desk handler. The officer must not be the applicant; the note is
// required; an approval may be for less than was asked, never more.
func (s *Service) DecideCreditApplication(ctx context.Context, req *connect.Request[lendingv1.DecideCreditApplicationRequest]) (*connect.Response[lendingv1.DecideCreditApplicationResponse], error) {
	m := req.Msg
	if m.FacilityId == "" || m.Officer == "" || strings.TrimSpace(m.Note) == "" || len(m.Note) > 1000 {
		return nil, invalid("facility_id, officer and a note of up to 1000 characters are required")
	}
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		var status, product, appliedBy string
		var requested int64
		var rate int32
		err := tx.QueryRow(ctx, `select status, product_id, applied_by, requested_minor, rate_bps from facility where facility_id = $1 for update`,
			m.FacilityId).Scan(&status, &product, &appliedBy, &requested, &rate)
		if errors.Is(err, pgx.ErrNoRows) {
			return connect.NewError(connect.CodeNotFound, errors.New("no such application"))
		}
		if err != nil {
			return err
		}
		if status != "ASSESSED" {
			return refused("this application has already been decided")
		}
		if strings.EqualFold(appliedBy, m.Officer) {
			return connect.NewError(connect.CodePermissionDenied, errors.New("the applicant cannot decide their own application"))
		}
		if !m.Approve {
			_, err := tx.Exec(ctx, `update facility set status = 'DECLINED', decided_by = $2, decided_at = $3, decision_note = $4
				where facility_id = $1`, m.FacilityId, m.Officer, s.now(), strings.TrimSpace(m.Note))
			return err
		}
		approved := requested
		if m.ApprovedMinor != "" {
			v, err := amountOf(m.ApprovedMinor)
			if err != nil {
				return err
			}
			if v > requested {
				return invalid("an approval cannot be for more than was asked")
			}
			approved = v
		}
		if m.RateBps != 0 {
			if m.RateBps > 10000 {
				return invalid("rate_bps must be at most 10000")
			}
			rate = int32(m.RateBps)
		}
		_, err = tx.Exec(ctx, `update facility set status = 'APPROVED', approved_minor = $2, rate_bps = $3, decided_by = $4,
			decided_at = $5, decision_note = $6 where facility_id = $1`, m.FacilityId, approved, rate, m.Officer, s.now(), strings.TrimSpace(m.Note))
		return err
	})
	if err != nil {
		return nil, wrap(err)
	}
	f, err := s.facility(ctx, "", m.FacilityId)
	if err != nil {
		return nil, wrap(err)
	}
	return connect.NewResponse(&lendingv1.DecideCreditApplicationResponse{Facility: f}), nil
}

// transfer posts legs as one linked chain; a refusal is the customer's (FailedPrecondition).
func (s *Service) transfer(ctx context.Context, key, actor, reason string, legs ...*ledgerv1.TransferLeg) error {
	txn, err := uuid.NewV5(namespace, key)
	if err != nil {
		return err
	}
	res, err := s.Ledger.CreateTransfers(ctx, connect.NewRequest(&ledgerv1.CreateTransfersRequest{
		IdempotencyKey: key, BusinessTxnId: txn, EntityCode: s.Entity, Legs: legs, Actor: actor, Reason: reason,
	}))
	if err != nil {
		return connect.NewError(connect.CodeUnavailable, err)
	}
	switch res.Msg.Status {
	case ledgerv1.Status_STATUS_OK, ledgerv1.Status_STATUS_REPLAYED:
		return nil
	}
	return connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("the ledger refused: %v", res.Msg.RejectionCodes))
}

// event records a money movement for statements. Its key is the movement's ledger key, so a retry records it once.
func (s *Service) event(ctx context.Context, q interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}, key, facility, kind string, principal, interest int64, desc string) error {
	_, err := q.Exec(ctx, `insert into facility_event (event_key, facility_id, at, kind, principal_minor, interest_minor, description)
		values ($1, $2, $3, $4, $5, $6, $7) on conflict (event_key) do nothing`, key, facility, s.now(), kind, principal, interest, desc)
	return err
}

func leg(from, to string, amount int64, code uint32) *ledgerv1.TransferLeg {
	return &ledgerv1.TransferLeg{DebitAccountId: from, CreditAccountId: to, Code: code,
		Amount: &ledgerv1.Money{Asset: "ZAR", AmountMinor: strconv.FormatInt(amount, 10)}}
}

// AcceptOffer implements the handler: the customer accepts an approved offer and the facility is opened. The loan
// account is keyed by the facility, and the payout by the facility, so a repeat or a crash replays and never pays twice.
func (s *Service) AcceptOffer(ctx context.Context, req *connect.Request[lendingv1.AcceptOfferRequest]) (*connect.Response[lendingv1.AcceptOfferResponse], error) {
	m := req.Msg
	if m.PartyId == "" || m.FacilityId == "" || m.Actor == "" {
		return nil, invalid("party_id, facility_id and actor are required")
	}
	if !m.AcceptsTerms {
		return nil, invalid("the quotation and terms must be accepted")
	}
	err := pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		var status, product, current, loan string
		var approved int64
		var rate, term, fee int32
		err := tx.QueryRow(ctx, `select status, product_id, current_account_id, loan_account_id, approved_minor, rate_bps, term_months, fee_bps
			from facility where facility_id = $1 and party_id = $2 for update`, m.FacilityId, m.PartyId).
			Scan(&status, &product, &current, &loan, &approved, &rate, &term, &fee)
		if errors.Is(err, pgx.ErrNoRows) {
			return connect.NewError(connect.CodeNotFound, errors.New("no such offer"))
		}
		if err != nil {
			return err
		}
		if status == "ACTIVE" || status == "SETTLED" {
			return nil // accepted already
		}
		if status != "APPROVED" {
			return refused("there is no approved offer to accept")
		}
		p, _ := s.product(product)
		if err := s.Licence.Allow(p.Licence); err != nil {
			return connect.NewError(connect.CodeFailedPrecondition, err)
		}
		if loan == "" {
			key := "facility-account:" + m.FacilityId
			txn, _ := uuid.NewV5(namespace, key)
			res, err := s.Ledger.CreateAccounts(ctx, connect.NewRequest(&ledgerv1.CreateAccountsRequest{
				IdempotencyKey: key, BusinessTxnId: txn, EntityCode: s.Entity, Actor: m.Actor, Reason: "open " + product,
				Accounts: []*ledgerv1.AccountSpec{{OwnerPartyId: m.PartyId, Asset: "ZAR", Code: 1020, ProductId: product}},
			}))
			if err != nil {
				return connect.NewError(connect.CodeUnavailable, err)
			}
			if len(res.Msg.AccountIds) != 1 {
				return refused("the ledger refused the loan account")
			}
			loan = res.Msg.AccountIds[0]
		}
		now := s.now()
		var feeOwed int64
		var after uint64
		switch product {
		case "term-loan":
			sched, err := credit.Schedule(approved, uint32(rate), int(term))
			if err != nil {
				return invalid(err.Error())
			}
			for _, i := range sched {
				due := monthsAfter(now, i.Number)
				if _, err := tx.Exec(ctx, `insert into instalment (facility_id, number, due_on, principal_minor, interest_minor, status)
					values ($1, $2, $3, $4, $5, 'DUE') on conflict do nothing`, m.FacilityId, i.Number, due, i.Principal, i.Interest); err != nil {
					return err
				}
			}
		case "revenue-based":
			feeOwed = credit.Fee(approved, uint32(fee))
			if _, after, err = s.inflow90d(ctx, m.PartyId, current); err != nil {
				return connect.NewError(connect.CodeUnavailable, err)
			}
		}
		if product != "overdraft" {
			if err := s.transfer(ctx, "disburse:"+m.FacilityId, m.Actor, "disburse "+product, leg(loan, current, approved, codeDisbursement)); err != nil {
				return err
			}
			if err := s.event(ctx, tx, "disburse:"+m.FacilityId, m.FacilityId, "DISBURSED", approved, 0, "paid into the current account"); err != nil {
				return err
			}
		}
		_, err = tx.Exec(ctx, `update facility set status = 'ACTIVE', loan_account_id = $2, accepted_at = $3, fee_owed_minor = $4,
			sweep_after_seq = $5 where facility_id = $1`, m.FacilityId, loan, now, feeOwed, int64(after))
		return err
	})
	if err != nil {
		return nil, wrap(err)
	}
	f, err := s.facility(ctx, m.PartyId, m.FacilityId)
	if err != nil {
		return nil, wrap(err)
	}
	return connect.NewResponse(&lendingv1.AcceptOfferResponse{Facility: f}), nil
}

// monthsAfter is the same day n months on, held to the 28th so every month has it.
func monthsAfter(t time.Time, n int) time.Time {
	day := min(t.Day(), 28)
	return time.Date(t.Year(), t.Month()+time.Month(n), day, 0, 0, 0, 0, time.UTC)
}

// lockActive locks an active facility the party owns and returns what a draw or repayment needs.
type active struct {
	product, current, loan string
	approved, feeOwed      int64
}

func lockActive(ctx context.Context, tx pgx.Tx, party, id string) (active, error) {
	var a active
	var status string
	err := tx.QueryRow(ctx, `select status, product_id, current_account_id, loan_account_id, approved_minor, fee_owed_minor
		from facility where facility_id = $1 and party_id = $2 for update`, id, party).Scan(&status, &a.product, &a.current, &a.loan, &a.approved, &a.feeOwed)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, connect.NewError(connect.CodeNotFound, errors.New("no such facility"))
	}
	if err != nil {
		return a, err
	}
	if status != "ACTIVE" {
		return a, refused("the facility is not active")
	}
	return a, nil
}

// seen records an operation's key; it returns true when the same request was already done.
func seen(ctx context.Context, tx pgx.Tx, key, h string) (bool, error) {
	var got string
	err := tx.QueryRow(ctx, `select request_hash from op where idempotency_key = $1`, key).Scan(&got)
	if errors.Is(err, pgx.ErrNoRows) {
		_, err = tx.Exec(ctx, `insert into op (idempotency_key, request_hash) values ($1, $2)`, key, h)
		return false, err
	}
	if err != nil {
		return false, err
	}
	if got != h {
		return false, connect.NewError(connect.CodeAlreadyExists, errors.New("this idempotency key was used for a different request"))
	}
	return true, nil
}

// Draw implements the handler: an overdraft draw, up to the limit, checked under the facility's lock.
func (s *Service) Draw(ctx context.Context, req *connect.Request[lendingv1.DrawRequest]) (*connect.Response[lendingv1.DrawResponse], error) {
	m := req.Msg
	if m.IdempotencyKey == "" || m.PartyId == "" || m.FacilityId == "" || m.Actor == "" {
		return nil, invalid("idempotency_key, party_id, facility_id and actor are required")
	}
	amount, err := amountOf(m.AmountMinor)
	if err != nil {
		return nil, err
	}
	key := "lending:" + m.PartyId + ":" + m.IdempotencyKey
	err = pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		a, err := lockActive(ctx, tx, m.PartyId, m.FacilityId)
		if err != nil {
			return err
		}
		if a.product != "overdraft" {
			return refused("only an overdraft facility can be drawn on")
		}
		if done, err := seen(ctx, tx, key, hash("draw", m.FacilityId, m.AmountMinor)); err != nil || done {
			return err
		}
		bal, err := s.balance(ctx, a.loan)
		if err != nil {
			return connect.NewError(connect.CodeUnavailable, err)
		}
		if -bal+amount > a.approved {
			return refused("the draw is more than the facility has left")
		}
		if err := s.transfer(ctx, key, m.Actor, "overdraft draw", leg(a.loan, a.current, amount, codeDisbursement)); err != nil {
			return err
		}
		return s.event(ctx, tx, key, m.FacilityId, "DRAWN", amount, 0, "drawn into the current account")
	})
	if err != nil {
		return nil, wrap(err)
	}
	f, err := s.facility(ctx, m.PartyId, m.FacilityId)
	if err != nil {
		return nil, wrap(err)
	}
	return connect.NewResponse(&lendingv1.DrawResponse{Facility: f}), nil
}

// Repay implements the handler. An overdraft or revenue-based advance takes any amount up to what is owed (fee and
// principal in proportion); a term loan is settled early only in full, and its future interest is waived.
func (s *Service) Repay(ctx context.Context, req *connect.Request[lendingv1.RepayRequest]) (*connect.Response[lendingv1.RepayResponse], error) {
	m := req.Msg
	if m.IdempotencyKey == "" || m.PartyId == "" || m.FacilityId == "" || m.Actor == "" {
		return nil, invalid("idempotency_key, party_id, facility_id and actor are required")
	}
	amount, err := amountOf(m.AmountMinor)
	if err != nil {
		return nil, err
	}
	key := "lending:" + m.PartyId + ":" + m.IdempotencyKey
	err = pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		a, err := lockActive(ctx, tx, m.PartyId, m.FacilityId)
		if err != nil {
			return err
		}
		if done, err := seen(ctx, tx, key, hash("repay", m.FacilityId, m.AmountMinor)); err != nil || done {
			return err
		}
		bal, err := s.balance(ctx, a.loan)
		if err != nil {
			return connect.NewError(connect.CodeUnavailable, err)
		}
		principal := -bal
		if amount > principal+a.feeOwed {
			return refused("the amount is more than is owed")
		}
		if a.product == "term-loan" && amount != principal {
			return refused("a loan is settled early in full: repay the outstanding principal")
		}
		pp, fp := amount, int64(0)
		if a.feeOwed > 0 {
			pp, fp = credit.Sweep(amount, 10000, principal, a.feeOwed)
		}
		legs := []*ledgerv1.TransferLeg{}
		if pp > 0 {
			legs = append(legs, leg(a.current, a.loan, pp, codeRepayment))
		}
		if fp > 0 {
			legs = append(legs, leg(a.current, s.Income, fp, codeInterestCharge))
		}
		if err := s.transfer(ctx, key, m.Actor, "repayment", legs...); err != nil {
			return err
		}
		if err := s.event(ctx, tx, key, m.FacilityId, "REPAID", pp, fp, "early repayment"); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `update facility set fee_owed_minor = fee_owed_minor - $2 where facility_id = $1`, m.FacilityId, fp); err != nil {
			return err
		}
		if a.product == "term-loan" {
			if _, err := tx.Exec(ctx, `update instalment set status = 'WAIVED' where facility_id = $1 and status in ('DUE', 'ARREARS')`, m.FacilityId); err != nil {
				return err
			}
		}
		if pp == principal && fp == a.feeOwed && a.product != "overdraft" {
			_, err = tx.Exec(ctx, `update facility set status = 'SETTLED', days_past_due = 0 where facility_id = $1`, m.FacilityId)
		}
		return err
	})
	if err != nil {
		return nil, wrap(err)
	}
	f, err := s.facility(ctx, m.PartyId, m.FacilityId)
	if err != nil {
		return nil, wrap(err)
	}
	return connect.NewResponse(&lendingv1.RepayResponse{Facility: f}), nil
}
