// Package acquiring runs payment links and point-of-sale payment requests, the acquirer's results, next-day
// settlement to the merchant's account, and refunds.
package acquiring

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	_ "embed"
	"encoding/base32"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	acquiringv1 "github.com/khayaai/forge-mutual/pkg/api/forge/acquiring/v1"
	ledgerv1 "github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1/ledgerv1connect"
	"github.com/khayaai/forge-mutual/pkg/licence"
	"github.com/khayaai/forge-mutual/pkg/systemaccounts"
	"github.com/khayaai/forge-mutual/pkg/uuid"
)

//go:embed schema.sql
var schema string

// Open applies the schema.
func Open(ctx context.Context, pool *pgxpool.Pool) error {
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `select pg_advisory_xact_lock(7754)`); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, schema)
		return err
	})
}

// Ownership reports the accounts a party owns and their assets.
type Ownership interface {
	Accounts(ctx context.Context, party string) (map[string]string, error)
}

// Link is a payment link as the service holds it.
type Link struct {
	ID, Token, Party, Account, Description, Reference, Channel, Merchant, Status, AcquirerRef, SettlementID string
	Amount, Refunded                                                                                        int64
	Created, Expires                                                                                        time.Time
	Paid                                                                                                    *time.Time
}

const (
	codeSettlement = 52 // ACQUIRING_SETTLEMENT
	codeFee        = 30 // FEE
	codeReversal   = 90 // REVERSAL
	namespace      = "0e7b3d9a-5c21-5f84-9a63-2d8f1b4c7e05"
)

// Service implements AcquiringService and CheckoutService, the acquirer webhook and the settlement run.
type Service struct {
	Pool     *pgxpool.Pool
	Ledger   ledgerv1connect.LedgerServiceClient
	Owners   Ownership
	Acquirer Acquirer
	Licence  licence.Gate
	Entity   string
	MDRBps   uint32 // merchant discount rate; a placeholder (docs/THRESHOLDS.md)
	Log      *slog.Logger
	Now      func() time.Time
	// System accounts, set by EnsureSystemAccounts.
	Settlement, Fees string
}

// EnsureSystemAccounts creates (or finds) ACQUIRER_SETTLEMENT and the acquiring fees account.
func (s *Service) EnsureSystemAccounts(ctx context.Context) error {
	var err error
	if s.Settlement, err = systemaccounts.Ensure(ctx, s.Ledger, s.Entity, "ZAR", 3020, "system:acquirer-settlement:ZAR:v1"); err != nil {
		return err
	}
	s.Fees, err = systemaccounts.Ensure(ctx, s.Ledger, s.Entity, "ZAR", 2002, "system:acquiring-fees:ZAR:v1")
	return err
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func invalid(msg string) error { return connect.NewError(connect.CodeInvalidArgument, errors.New(msg)) }

func wrap(err error) error {
	var ce *connect.Error
	if err == nil || errors.As(err, &ce) {
		return err
	}
	return connect.NewError(connect.CodeInternal, fmt.Errorf("acquiring: %w", err))
}

const linkColumns = `link_id, token, party_id, settlement_account_id, amount_minor, description, reference, channel, merchant_name,
	status, created_at, expires_at, paid_at, acquirer_ref, refunded_minor, settlement_id`

func scanLink(row pgx.Row) (Link, error) {
	var l Link
	err := row.Scan(&l.ID, &l.Token, &l.Party, &l.Account, &l.Amount, &l.Description, &l.Reference, &l.Channel, &l.Merchant,
		&l.Status, &l.Created, &l.Expires, &l.Paid, &l.AcquirerRef, &l.Refunded, &l.SettlementID)
	return l, err
}

func (l Link) proto(now time.Time) *acquiringv1.PaymentLink {
	status := l.Status
	if status == "OPEN" && now.After(l.Expires) {
		status = "EXPIRED"
	}
	p := &acquiringv1.PaymentLink{
		LinkId: l.ID, Token: l.Token, Channel: l.Channel, AmountMinor: strconv.FormatInt(l.Amount, 10), Description: l.Description,
		Reference: l.Reference, Status: status, CreatedAt: l.Created.UTC().Format(time.RFC3339), ExpiresAt: l.Expires.UTC().Format(time.RFC3339),
		RefundedMinor: strconv.FormatInt(l.Refunded, 10), SettlementAccountId: l.Account, MerchantName: l.Merchant,
	}
	if l.Paid != nil {
		p.PaidAt = l.Paid.UTC().Format(time.RFC3339)
	}
	return p
}

// newToken is 20 random bytes in base32: unguessable, and safe in a URL.
func newToken() (string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b)), nil
}

// CreatePaymentLink implements the handler.
func (s *Service) CreatePaymentLink(ctx context.Context, req *connect.Request[acquiringv1.CreatePaymentLinkRequest]) (*connect.Response[acquiringv1.CreatePaymentLinkResponse], error) {
	m := req.Msg
	if err := s.Licence.Allow(licence.Acquiring); err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	if m.IdempotencyKey == "" || m.PartyId == "" || m.Actor == "" {
		return nil, invalid("idempotency_key, party_id and actor are required")
	}
	amount, err := strconv.ParseInt(m.AmountMinor, 10, 64)
	if err != nil || amount <= 0 || amount > 100_000_000 {
		return nil, invalid("amount_minor must be a positive integer up to R1 000 000.00")
	}
	desc, merchant := strings.TrimSpace(m.Description), strings.TrimSpace(m.MerchantName)
	if desc == "" || len(desc) > 140 || merchant == "" || len(merchant) > 80 || len(m.Reference) > 35 {
		return nil, invalid("a description (up to 140), a merchant name (up to 80) and a reference of up to 35 characters are required")
	}
	channel := m.Channel
	if channel == "" {
		channel = "link"
	}
	if channel != "link" && channel != "pos" {
		return nil, invalid(`channel must be "link" or "pos"`)
	}
	hours := int(m.ValidHours)
	if hours == 0 {
		hours = map[string]int{"link": 168, "pos": 1}[channel]
	}
	if hours < 1 || hours > 24*90 {
		return nil, invalid("valid_hours must be 1 to 2160")
	}
	owned, err := s.Owners.Accounts(ctx, m.PartyId)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	if owned[m.SettlementAccountId] != "ZAR" {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("takings settle to the party's own rand account"))
	}
	key := "acquiring:" + m.PartyId + ":" + m.IdempotencyKey
	id, err := uuid.NewV5(namespace, key)
	if err != nil {
		return nil, wrap(err)
	}
	token, err := newToken()
	if err != nil {
		return nil, wrap(err)
	}
	now := s.now()
	if _, err := s.Pool.Exec(ctx, `insert into payment_link (link_id, token, party_id, settlement_account_id, amount_minor, description,
		reference, channel, merchant_name, status, created_at, expires_at, idempotency_key)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'OPEN', $10, $11, $12) on conflict (idempotency_key) do nothing`,
		id, token, m.PartyId, m.SettlementAccountId, amount, desc, strings.TrimSpace(m.Reference), channel, merchant, now,
		now.Add(time.Duration(hours)*time.Hour), key); err != nil {
		return nil, wrap(err)
	}
	l, err := scanLink(s.Pool.QueryRow(ctx, `select `+linkColumns+` from payment_link where link_id = $1 and party_id = $2`, id, m.PartyId))
	if err != nil {
		return nil, wrap(err)
	}
	return connect.NewResponse(&acquiringv1.CreatePaymentLinkResponse{Link: l.proto(now)}), nil
}

// ListPaymentLinks implements the handler.
func (s *Service) ListPaymentLinks(ctx context.Context, req *connect.Request[acquiringv1.ListPaymentLinksRequest]) (*connect.Response[acquiringv1.ListPaymentLinksResponse], error) {
	if req.Msg.PartyId == "" {
		return nil, invalid("party_id is required")
	}
	rows, err := s.Pool.Query(ctx, `select `+linkColumns+` from payment_link where party_id = $1 order by created_at desc limit 200`, req.Msg.PartyId)
	if err != nil {
		return nil, wrap(err)
	}
	defer rows.Close()
	out := &acquiringv1.ListPaymentLinksResponse{}
	now := s.now()
	for rows.Next() {
		l, err := scanLink(rows)
		if err != nil {
			return nil, wrap(err)
		}
		out.Links = append(out.Links, l.proto(now))
	}
	return connect.NewResponse(out), rows.Err()
}

// CancelPaymentLink implements the handler. Only an open link can be cancelled.
func (s *Service) CancelPaymentLink(ctx context.Context, req *connect.Request[acquiringv1.CancelPaymentLinkRequest]) (*connect.Response[acquiringv1.CancelPaymentLinkResponse], error) {
	l, err := scanLink(s.Pool.QueryRow(ctx, `update payment_link set status = 'CANCELLED' where link_id = $1 and party_id = $2 and status = 'OPEN'
		returning `+linkColumns, req.Msg.LinkId, req.Msg.PartyId))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("no open link with that id"))
	}
	if err != nil {
		return nil, wrap(err)
	}
	return connect.NewResponse(&acquiringv1.CancelPaymentLinkResponse{Link: l.proto(s.now())}), nil
}

// --- the payer's side ---

func (s *Service) byToken(ctx context.Context, token string) (Link, error) {
	if token == "" || len(token) > 64 {
		return Link{}, connect.NewError(connect.CodeNotFound, errors.New("no such payment"))
	}
	l, err := scanLink(s.Pool.QueryRow(ctx, `select `+linkColumns+` from payment_link where token = $1`, token))
	if errors.Is(err, pgx.ErrNoRows) {
		return Link{}, connect.NewError(connect.CodeNotFound, errors.New("no such payment"))
	}
	return l, err
}

// GetCheckout implements the handler. It shows the payer only what they are paying and to whom.
func (s *Service) GetCheckout(ctx context.Context, req *connect.Request[acquiringv1.GetCheckoutRequest]) (*connect.Response[acquiringv1.GetCheckoutResponse], error) {
	l, err := s.byToken(ctx, req.Msg.Token)
	if err != nil {
		return nil, wrap(err)
	}
	p := l.proto(s.now())
	return connect.NewResponse(&acquiringv1.GetCheckoutResponse{MerchantName: l.Merchant, AmountMinor: p.AmountMinor,
		Description: l.Description, Reference: l.Reference, Status: p.Status, ExpiresAt: p.ExpiresAt}), nil
}

// StartCheckout implements the handler: it sends the payer to the acquirer's hosted checkout.
func (s *Service) StartCheckout(ctx context.Context, req *connect.Request[acquiringv1.StartCheckoutRequest]) (*connect.Response[acquiringv1.StartCheckoutResponse], error) {
	if err := s.Licence.Allow(licence.Acquiring); err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	l, err := s.byToken(ctx, req.Msg.Token)
	if err != nil {
		return nil, wrap(err)
	}
	if l.proto(s.now()).Status != "OPEN" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("this payment can no longer be made"))
	}
	url, sim, err := s.Acquirer.Checkout(ctx, l)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	return connect.NewResponse(&acquiringv1.StartCheckoutResponse{CheckoutUrl: url, Simulated: sim}), nil
}

// SimulatePayment implements the handler: the simulator's checkout reports a result as its webhook would.
func (s *Service) SimulatePayment(ctx context.Context, req *connect.Request[acquiringv1.SimulatePaymentRequest]) (*connect.Response[acquiringv1.SimulatePaymentResponse], error) {
	if !s.Acquirer.Simulated() {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("the acquirer simulator is not in use"))
	}
	l, err := s.byToken(ctx, req.Msg.Token)
	if err != nil {
		return nil, wrap(err)
	}
	status := "DECLINED"
	if req.Msg.Approve {
		status = "APPROVED"
	}
	if err := s.record(ctx, Result{LinkID: l.ID, Status: status, AmountMinor: l.Amount, Reference: "sim-" + l.ID[:8]}); err != nil {
		return nil, wrap(err)
	}
	return connect.NewResponse(&acquiringv1.SimulatePaymentResponse{Status: status}), nil
}

// record applies an acquirer result. An approval marks the link paid, once; it must be for the link's amount. A
// decline leaves the link open for another attempt.
func (s *Service) record(ctx context.Context, r Result) error {
	if r.Status != "APPROVED" {
		return nil
	}
	tag, err := s.Pool.Exec(ctx, `update payment_link set status = 'PAID', paid_at = $3, acquirer_ref = $4
		where link_id = $1 and amount_minor = $2 and status in ('OPEN', 'EXPIRED')`, r.LinkID, r.AmountMinor, s.now(), r.Reference)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		var status string
		var amount int64
		err := s.Pool.QueryRow(ctx, `select status, amount_minor from payment_link where link_id = $1`, r.LinkID).Scan(&status, &amount)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return connect.NewError(connect.CodeNotFound, errors.New("no such link"))
		case err != nil:
			return err
		case status == "PAID":
			return nil // a repeat delivery
		case amount != r.AmountMinor:
			return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("the acquirer approved %d for a link of %d", r.AmountMinor, amount))
		default:
			return connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("an approval for a %s link", strings.ToLower(status)))
		}
	}
	return nil
}

// Webhook is the acquirer's HTTP endpoint. It answers 401 to anything the acquirer did not sign.
func (s *Service) Webhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
	if err != nil {
		http.Error(w, "too large", http.StatusRequestEntityTooLarge)
		return
	}
	res, err := s.Acquirer.VerifyWebhook(body, r.Header)
	if err != nil {
		http.Error(w, "unauthorised", http.StatusUnauthorized)
		return
	}
	if err := s.record(r.Context(), res); err != nil {
		if s.Log != nil {
			s.Log.Error("acquirer result not recorded", "link", res.LinkID, "status", res.Status, "err", err)
		}
		code := http.StatusServiceUnavailable
		if c := connect.CodeOf(err); c == connect.CodeInvalidArgument || c == connect.CodeFailedPrecondition || c == connect.CodeNotFound {
			code = http.StatusUnprocessableEntity // recorded in the log for operations; the acquirer need not retry
		}
		http.Error(w, "not recorded", code)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// --- settlement and refunds ---

func (s *Service) transfer(ctx context.Context, key, reason string, legs ...*ledgerv1.TransferLeg) error {
	txn, err := uuid.NewV5(namespace, key)
	if err != nil {
		return err
	}
	res, err := s.Ledger.CreateTransfers(ctx, connect.NewRequest(&ledgerv1.CreateTransfersRequest{
		IdempotencyKey: key, BusinessTxnId: txn, EntityCode: s.Entity, Legs: legs, Actor: "acquiring-service", Reason: reason,
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

func leg(from, to string, amount int64, code uint32) *ledgerv1.TransferLeg {
	return &ledgerv1.TransferLeg{DebitAccountId: from, CreditAccountId: to, Code: code,
		Amount: &ledgerv1.Money{Asset: "ZAR", AmountMinor: strconv.FormatInt(amount, 10)}}
}

// Fee is the merchant discount on a day's takings, rounded down.
func Fee(gross int64, bps uint32) int64 { return gross * int64(bps) / 10000 }

// Settle pays each merchant, per settlement account, the takings of every day before today: gross from acquirer
// settlement, less the merchant discount to fees, as one chain under the settlement's key. Grouping is recorded first,
// so a crash finishes with the same settlement and pays once.
func (s *Service) Settle(ctx context.Context) (int, error) {
	today := s.now().Truncate(24 * time.Hour)
	if _, err := s.Pool.Exec(ctx, `
		with days as (
		  select party_id, settlement_account_id, (paid_at at time zone 'UTC')::date as day, sum(amount_minor) as gross, count(*) as n
		  from payment_link where status = 'PAID' and settlement_id = '' and paid_at < $1
		  group by 1, 2, 3)
		insert into settlement (settlement_id, party_id, account_id, day, gross_minor, fee_minor, net_minor, payments, status)
		select md5(party_id || ':' || settlement_account_id || ':' || day::text), party_id, settlement_account_id, day, gross,
		       gross * $2 / 10000, gross - gross * $2 / 10000, n, 'PENDING'
		from days on conflict (party_id, account_id, day) do nothing`, today, int64(s.MDRBps)); err != nil {
		return 0, err
	}
	if _, err := s.Pool.Exec(ctx, `update payment_link l set settlement_id = st.settlement_id from settlement st
		where l.status = 'PAID' and l.settlement_id = '' and l.paid_at < $1 and st.party_id = l.party_id
		and st.account_id = l.settlement_account_id and st.day = (l.paid_at at time zone 'UTC')::date`, today); err != nil {
		return 0, err
	}
	rows, err := s.Pool.Query(ctx, `select settlement_id, account_id, gross_minor, fee_minor from settlement where status = 'PENDING' order by day, settlement_id`)
	if err != nil {
		return 0, err
	}
	type st struct {
		id, account string
		gross, fee  int64
	}
	var todo []st
	for rows.Next() {
		var x st
		if err := rows.Scan(&x.id, &x.account, &x.gross, &x.fee); err != nil {
			rows.Close()
			return 0, err
		}
		todo = append(todo, x)
	}
	rows.Close()
	paid := 0
	for _, x := range todo {
		legs := []*ledgerv1.TransferLeg{leg(s.Settlement, x.account, x.gross, codeSettlement)}
		if x.fee > 0 {
			legs = append(legs, leg(x.account, s.Fees, x.fee, codeFee))
		}
		if err := s.transfer(ctx, "settle:"+x.id, "card settlement", legs...); err != nil {
			return paid, err
		}
		if _, err := s.Pool.Exec(ctx, `update settlement set status = 'PAID' where settlement_id = $1`, x.id); err != nil {
			return paid, err
		}
		paid++
	}
	return paid, nil
}

// ListSettlements implements the handler.
func (s *Service) ListSettlements(ctx context.Context, req *connect.Request[acquiringv1.ListSettlementsRequest]) (*connect.Response[acquiringv1.ListSettlementsResponse], error) {
	rows, err := s.Pool.Query(ctx, `select settlement_id, day, gross_minor, fee_minor, net_minor, payments, status from settlement
		where party_id = $1 order by day desc limit 90`, req.Msg.PartyId)
	if err != nil {
		return nil, wrap(err)
	}
	defer rows.Close()
	out := &acquiringv1.ListSettlementsResponse{MdrBps: s.MDRBps}
	for rows.Next() {
		var x acquiringv1.Settlement
		var day time.Time
		var g, f, n int64
		var p int32
		if err := rows.Scan(&x.SettlementId, &day, &g, &f, &n, &p, &x.Status); err != nil {
			return nil, wrap(err)
		}
		x.Day, x.GrossMinor, x.FeeMinor, x.NetMinor, x.Payments = day.Format("2006-01-02"), fmt.Sprint(g), fmt.Sprint(f), fmt.Sprint(n), uint32(p)
		out.Settlements = append(out.Settlements, &x)
	}
	return connect.NewResponse(out), rows.Err()
}

// RefundPayment implements the handler. A payment is refunded once it has settled, up to what was paid less what was
// refunded already. The merchant's account is debited first, under the refund's key; then the acquirer refunds the
// card. A retry with the same key finishes a refund the acquirer did not confirm, and never debits twice.
func (s *Service) RefundPayment(ctx context.Context, req *connect.Request[acquiringv1.RefundPaymentRequest]) (*connect.Response[acquiringv1.RefundPaymentResponse], error) {
	m := req.Msg
	amount, err := strconv.ParseInt(m.AmountMinor, 10, 64)
	if m.IdempotencyKey == "" || m.PartyId == "" || m.LinkId == "" || err != nil || amount <= 0 {
		return nil, invalid("idempotency_key, party_id, link_id and a positive amount_minor are required")
	}
	key := "refund:" + m.PartyId + ":" + m.IdempotencyKey
	h := sha256.Sum256([]byte(m.LinkId + "\x00" + m.AmountMinor))
	hash := hex.EncodeToString(h[:])
	var link Link
	var send bool
	err = pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		l, err := scanLink(tx.QueryRow(ctx, `select `+linkColumns+` from payment_link where link_id = $1 and party_id = $2 for update`, m.LinkId, m.PartyId))
		if errors.Is(err, pgx.ErrNoRows) {
			return connect.NewError(connect.CodeNotFound, errors.New("no such payment"))
		}
		if err != nil {
			return err
		}
		link = l
		var got, status string
		err = tx.QueryRow(ctx, `select request_hash, status from refund where idempotency_key = $1`, key).Scan(&got, &status)
		if err == nil {
			if got != hash {
				return connect.NewError(connect.CodeAlreadyExists, errors.New("this idempotency key was used for a different refund"))
			}
			send = status == "BOOKED"
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		var settled string
		if l.SettlementID != "" {
			_ = tx.QueryRow(ctx, `select status from settlement where settlement_id = $1`, l.SettlementID).Scan(&settled)
		}
		if l.Status != "PAID" || settled != "PAID" {
			return connect.NewError(connect.CodeFailedPrecondition, errors.New("a payment can be refunded once it has settled"))
		}
		if amount > l.Amount-l.Refunded {
			return connect.NewError(connect.CodeFailedPrecondition, errors.New("the refund is more than is left to refund"))
		}
		if err := s.transfer(ctx, key, "refund", leg(l.Account, s.Settlement, amount, codeReversal)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `insert into refund (idempotency_key, link_id, amount_minor, request_hash, status, created_at)
			values ($1, $2, $3, $4, 'BOOKED', $5)`, key, l.ID, amount, hash, s.now()); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `update payment_link set refunded_minor = refunded_minor + $2 where link_id = $1`, l.ID, amount)
		send = true
		return err
	})
	if err != nil {
		return nil, wrap(err)
	}
	if send {
		if err := s.Acquirer.Refund(ctx, link, amount, key); err != nil {
			return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("the refund is booked; the acquirer did not confirm it, so send the same request again: %w", err))
		}
		if _, err := s.Pool.Exec(ctx, `update refund set status = 'SENT' where idempotency_key = $1`, key); err != nil {
			return nil, wrap(err)
		}
	}
	l, err := scanLink(s.Pool.QueryRow(ctx, `select `+linkColumns+` from payment_link where link_id = $1`, m.LinkId))
	if err != nil {
		return nil, wrap(err)
	}
	return connect.NewResponse(&acquiringv1.RefundPaymentResponse{Link: l.proto(s.now())}), nil
}
