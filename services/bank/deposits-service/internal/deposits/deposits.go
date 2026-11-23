// Package deposits runs savings and notice deposit accounts. The money is in the ledger: a deposit account is a
// customer deposit account (code 1001) with a deposit product. This package decides who may move what, when, and
// computes and pays the interest.
package deposits

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	depositsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/deposits/v1"
	ledgerv1 "github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1/ledgerv1connect"
	"github.com/khayaai/forge-mutual/pkg/licence"
	"github.com/khayaai/forge-mutual/pkg/uuid"
)

//go:embed schema.sql
var schema string

// Open applies the schema.
func Open(ctx context.Context, pool *pgxpool.Pool) error {
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `select pg_advisory_xact_lock(7752)`); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, schema)
		return err
	})
}

// Product is one deposit product. Rates are placeholders until treasury confirms them (docs/THRESHOLDS.md).
type Product struct {
	ID         string
	Name       string
	RateBps    uint32
	NoticeDays int
	Licence    licence.Product
}

// DefaultProducts are the products offered, with placeholder rates.
func DefaultProducts() []Product {
	return []Product{
		{ID: "savings", Name: "Savings", RateBps: 650, Licence: licence.Savings},
		{ID: "notice-32", Name: "32-day notice", RateBps: 775, NoticeDays: 32, Licence: licence.NoticeDeposits},
		{ID: "notice-90", Name: "90-day notice", RateBps: 850, NoticeDays: 90, Licence: licence.NoticeDeposits},
	}
}

// Ownership reports the accounts a party owns and their assets (consolidated-view).
type Ownership interface {
	Accounts(ctx context.Context, party string) (map[string]string, error)
}

// Ledger transfer codes.
const (
	codeInternal       = 12 // INTERNAL_TRANSFER
	codeInterestCredit = 65 // INTEREST_CREDIT
	namespace          = "9d4e7c1a-2b5f-5a38-8e61-3c0f9b7d2a46"
)

// Service implements depositsv1connect.DepositsServiceHandler, and runs the daily and monthly work.
type Service struct {
	Pool     *pgxpool.Pool
	Ledger   ledgerv1connect.LedgerServiceClient
	Owners   Ownership
	Products []Product
	Licence  licence.Gate
	Entity   string
	Insured  bool // the deposit-insurance flag for deposit accounts (INSURED_DEPOSITS)
	// Expense is the INTEREST_EXPENSE system account interest is paid from. Set it with EnsureSystemAccounts.
	Expense string
	Now     func() time.Time
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
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

func internal(err error) error {
	var ce *connect.Error
	if errors.As(err, &ce) {
		return ce
	}
	return connect.NewError(connect.CodeInternal, fmt.Errorf("deposits: %w", err))
}

// ListProducts implements the handler.
func (s *Service) ListProducts(context.Context, *connect.Request[depositsv1.ListProductsRequest]) (*connect.Response[depositsv1.ListProductsResponse], error) {
	out := &depositsv1.ListProductsResponse{}
	for _, p := range s.Products {
		out.Products = append(out.Products, &depositsv1.Product{
			ProductId: p.ID, Name: p.Name, RateBps: p.RateBps, NoticeDays: uint32(p.NoticeDays), Available: s.Licence.Allow(p.Licence) == nil,
		})
	}
	return connect.NewResponse(out), nil
}

// OpenDepositAccount implements the handler. The ledger account is keyed by party and product, so a repeat replays.
func (s *Service) OpenDepositAccount(ctx context.Context, req *connect.Request[depositsv1.OpenDepositAccountRequest]) (*connect.Response[depositsv1.OpenDepositAccountResponse], error) {
	m := req.Msg
	p, ok := s.product(m.ProductId)
	if m.PartyId == "" || m.Actor == "" || !ok {
		return nil, invalid("party_id, actor and a listed product_id are required")
	}
	if err := s.Licence.Allow(p.Licence); err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	// Only a customer with a current account may open a deposit account: it is where deposits come from.
	owned, err := s.Owners.Accounts(ctx, m.PartyId)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	hasCurrent := false
	for _, asset := range owned {
		hasCurrent = hasCurrent || asset == "ZAR"
	}
	if !hasCurrent {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("open a rand current account first"))
	}
	key := "deposit-account:" + m.PartyId + ":" + p.ID
	txn, err := uuid.NewV5(namespace, key)
	if err != nil {
		return nil, internal(err)
	}
	res, err := s.Ledger.CreateAccounts(ctx, connect.NewRequest(&ledgerv1.CreateAccountsRequest{
		IdempotencyKey: key, BusinessTxnId: txn, EntityCode: s.Entity, Actor: m.Actor, Reason: "open " + p.ID,
		Accounts: []*ledgerv1.AccountSpec{{
			OwnerPartyId: m.PartyId, Asset: "ZAR", Code: 1001, ProductId: p.ID, DebitsMustNotExceedCredits: true, Insured: s.Insured,
		}},
	}))
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	if (res.Msg.Status != ledgerv1.Status_STATUS_OK && res.Msg.Status != ledgerv1.Status_STATUS_REPLAYED) || len(res.Msg.AccountIds) != 1 {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("the ledger refused the account: %v", res.Msg.RejectionCodes))
	}
	id := res.Msg.AccountIds[0]
	if _, err := s.Pool.Exec(ctx, `insert into deposit_account (account_id, party_id, product_id, opened_at) values ($1, $2, $3, $4)
		on conflict do nothing`, id, m.PartyId, p.ID, s.now().UTC()); err != nil {
		return nil, internal(err)
	}
	accs, err := s.accounts(ctx, m.PartyId, id)
	if err != nil || len(accs) != 1 {
		return nil, internal(fmt.Errorf("read back: %v", err))
	}
	return connect.NewResponse(&depositsv1.OpenDepositAccountResponse{Account: accs[0]}), nil
}

// ListDepositAccounts implements the handler.
func (s *Service) ListDepositAccounts(ctx context.Context, req *connect.Request[depositsv1.ListDepositAccountsRequest]) (*connect.Response[depositsv1.ListDepositAccountsResponse], error) {
	if req.Msg.PartyId == "" {
		return nil, invalid("party_id is required")
	}
	accs, err := s.accounts(ctx, req.Msg.PartyId, "")
	if err != nil {
		return nil, internal(err)
	}
	return connect.NewResponse(&depositsv1.ListDepositAccountsResponse{Accounts: accs}), nil
}

// accounts reads a party's deposit accounts (or one of them) with their balances, accruals and notices.
func (s *Service) accounts(ctx context.Context, party, only string) ([]*depositsv1.DepositAccount, error) {
	rows, err := s.Pool.Query(ctx, `select account_id, product_id, opened_at from deposit_account
		where party_id = $1 and ($2 = '' or account_id = $2) order by opened_at, account_id`, party, only)
	if err != nil {
		return nil, err
	}
	var out []*depositsv1.DepositAccount
	var ids []string
	for rows.Next() {
		var a depositsv1.DepositAccount
		var opened time.Time
		if err := rows.Scan(&a.AccountId, &a.ProductId, &opened); err != nil {
			rows.Close()
			return nil, err
		}
		a.OpenedAt = opened.UTC().Format(time.RFC3339)
		if p, ok := s.product(a.ProductId); ok {
			a.RateBps, a.NoticeDays = p.RateBps, uint32(p.NoticeDays)
		}
		out = append(out, &a)
		ids = append(ids, a.AccountId)
	}
	rows.Close()
	if len(ids) == 0 {
		return out, nil
	}
	bal, err := s.balances(ctx, ids)
	if err != nil {
		return nil, err
	}
	month := s.now().UTC().Format("2006-01")
	for _, a := range out {
		var accrued, carry, under, paid int64
		if err := s.Pool.QueryRow(ctx, `select coalesce(sum(accrued_nano), 0) from accrual
			where account_id = $1 and to_char(day, 'YYYY-MM') = $2`, a.AccountId, month).Scan(&accrued); err != nil {
			return nil, err
		}
		if err := s.Pool.QueryRow(ctx, `select coalesce((select carry_out_nano from interest_payment where account_id = $1
			order by period desc limit 1), 0), coalesce((select sum(paid_minor) from interest_payment where account_id = $1 and status = 'PAID'), 0)`,
			a.AccountId).Scan(&carry, &paid); err != nil {
			return nil, err
		}
		if err := s.Pool.QueryRow(ctx, `select coalesce(sum(amount_minor), 0) from notice where account_id = $1 and status = 'PENDING'`,
			a.AccountId).Scan(&under); err != nil {
			return nil, err
		}
		b := bal[a.AccountId]
		avail := b - under
		if avail < 0 {
			avail = 0
		}
		a.BalanceMinor = strconv.FormatInt(b, 10)
		a.AccruedMinor = strconv.FormatInt((accrued+carry)/1_000_000_000, 10)
		a.UnderNoticeMinor = strconv.FormatInt(under, 10)
		a.AvailableMinor = strconv.FormatInt(avail, 10)
		a.InterestPaidMinor = strconv.FormatInt(paid, 10)
		if a.Notices, err = s.notices(ctx, a.AccountId); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (s *Service) notices(ctx context.Context, account string) ([]*depositsv1.Notice, error) {
	rows, err := s.Pool.Query(ctx, `select notice_id, amount_minor, to_account_id, given_at, due_on, status from notice
		where account_id = $1 order by given_at desc limit 20`, account)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*depositsv1.Notice
	for rows.Next() {
		var n depositsv1.Notice
		var amount int64
		var given, due time.Time
		if err := rows.Scan(&n.NoticeId, &amount, &n.ToAccountId, &given, &due, &n.Status); err != nil {
			return nil, err
		}
		n.AmountMinor, n.GivenAt, n.DueOn = strconv.FormatInt(amount, 10), given.UTC().Format(time.RFC3339), due.Format("2006-01-02")
		out = append(out, &n)
	}
	return out, rows.Err()
}

// balances reads available balances from the ledger.
func (s *Service) balances(ctx context.Context, ids []string) (map[string]int64, error) {
	res, err := s.Ledger.GetBalances(ctx, connect.NewRequest(&ledgerv1.GetBalancesRequest{EntityCode: s.Entity, AccountIds: ids}))
	if err != nil {
		return nil, err
	}
	out := map[string]int64{}
	for _, b := range res.Msg.Balances {
		v, err := strconv.ParseInt(b.AvailableMinor, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("balance %s: %w", b.AccountId, err)
		}
		out[b.AccountId] = v
	}
	return out, nil
}

// owned returns the product of a deposit account the party owns, or NotFound.
func (s *Service) owned(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, party, account string, lock bool) (Product, error) {
	sql := `select product_id from deposit_account where account_id = $1 and party_id = $2`
	if lock {
		sql += ` for update`
	}
	var id string
	err := q.QueryRow(ctx, sql, account, party).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Product{}, connect.NewError(connect.CodeNotFound, errors.New("no such deposit account"))
	}
	if err != nil {
		return Product{}, err
	}
	p, ok := s.product(id)
	if !ok {
		return Product{}, fmt.Errorf("account %s has an unknown product %q", account, id)
	}
	return p, nil
}

func hash(parts ...string) string {
	h := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(h[:])
}

func parseAmount(s string) (int64, error) {
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil || v <= 0 {
		return 0, invalid("amount_minor must be a positive integer")
	}
	return v, nil
}

// replay returns a recorded move for the key, or nil. The same key with a different request is a conflict.
func replay(ctx context.Context, tx pgx.Tx, key, h string) (*depositsv1.WithdrawResponse, error) {
	var got, state, notice string
	err := tx.QueryRow(ctx, `select request_hash, state, notice_id from move where idempotency_key = $1`, key).Scan(&got, &state, &notice)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if got != h {
		return nil, connect.NewError(connect.CodeAlreadyExists, errors.New("this idempotency key was used for a different request"))
	}
	out := &depositsv1.WithdrawResponse{State: state}
	if notice != "" {
		var n depositsv1.Notice
		var amount int64
		var given, due time.Time
		if err := tx.QueryRow(ctx, `select notice_id, amount_minor, to_account_id, given_at, due_on, status from notice where notice_id = $1`, notice).
			Scan(&n.NoticeId, &amount, &n.ToAccountId, &given, &due, &n.Status); err != nil {
			return nil, err
		}
		n.AmountMinor, n.GivenAt, n.DueOn = strconv.FormatInt(amount, 10), given.UTC().Format(time.RFC3339), due.Format("2006-01-02")
		out.Notice = &n
	}
	return out, nil
}

// transfer moves money in the ledger as one internal transfer. Refusals are reported to the customer.
func (s *Service) transfer(ctx context.Context, key, from, to string, amount int64, code uint32, actor, reason string) error {
	txn, err := uuid.NewV5(namespace, key)
	if err != nil {
		return err
	}
	res, err := s.Ledger.CreateTransfers(ctx, connect.NewRequest(&ledgerv1.CreateTransfersRequest{
		IdempotencyKey: key, BusinessTxnId: txn, EntityCode: s.Entity, Actor: actor, Reason: reason,
		Legs: []*ledgerv1.TransferLeg{{DebitAccountId: from, CreditAccountId: to, Code: code,
			Amount: &ledgerv1.Money{Asset: "ZAR", AmountMinor: strconv.FormatInt(amount, 10)}}},
	}))
	if err != nil {
		return connect.NewError(connect.CodeUnavailable, err)
	}
	switch res.Msg.Status {
	case ledgerv1.Status_STATUS_OK, ledgerv1.Status_STATUS_REPLAYED:
		return nil
	}
	return connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("the ledger refused the transfer: %v", res.Msg.RejectionCodes))
}

// Deposit implements the handler: from the party's rand current account into its deposit account.
func (s *Service) Deposit(ctx context.Context, req *connect.Request[depositsv1.DepositRequest]) (*connect.Response[depositsv1.DepositResponse], error) {
	m := req.Msg
	if m.IdempotencyKey == "" || m.PartyId == "" || m.Actor == "" {
		return nil, invalid("idempotency_key, party_id and actor are required")
	}
	amount, err := parseAmount(m.AmountMinor)
	if err != nil {
		return nil, err
	}
	owned, err := s.Owners.Accounts(ctx, m.PartyId)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	if owned[m.FromAccountId] != "ZAR" {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("deposits come from the party's own rand account"))
	}
	key := "deposits:" + m.PartyId + ":" + m.IdempotencyKey
	h := hash("deposit", m.FromAccountId, m.ToAccountId, m.AmountMinor)
	var out *depositsv1.DepositResponse
	err = pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if r, err := replay(ctx, tx, key, h); err != nil || r != nil {
			if r != nil {
				out = &depositsv1.DepositResponse{State: r.State}
			}
			return err
		}
		if _, err := s.owned(ctx, tx, m.PartyId, m.ToAccountId, true); err != nil {
			return err
		}
		if err := s.transfer(ctx, key, m.FromAccountId, m.ToAccountId, amount, codeInternal, m.Actor, "deposit"); err != nil {
			return err
		}
		out = &depositsv1.DepositResponse{State: "MOVED"}
		_, err := tx.Exec(ctx, `insert into move (idempotency_key, request_hash, state) values ($1, $2, 'MOVED')`, key, h)
		return err
	})
	if err != nil {
		return nil, internal(err)
	}
	return connect.NewResponse(out), nil
}

// Withdraw implements the handler. Savings pays out now; a notice account takes notice. Either way only what is not
// already under notice may be withdrawn, and the account row is locked so two requests cannot both spend it.
func (s *Service) Withdraw(ctx context.Context, req *connect.Request[depositsv1.WithdrawRequest]) (*connect.Response[depositsv1.WithdrawResponse], error) {
	m := req.Msg
	if m.IdempotencyKey == "" || m.PartyId == "" || m.Actor == "" {
		return nil, invalid("idempotency_key, party_id and actor are required")
	}
	amount, err := parseAmount(m.AmountMinor)
	if err != nil {
		return nil, err
	}
	owned, err := s.Owners.Accounts(ctx, m.PartyId)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	if owned[m.ToAccountId] != "ZAR" {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("withdrawals go to the party's own rand account"))
	}
	key := "deposits:" + m.PartyId + ":" + m.IdempotencyKey
	h := hash("withdraw", m.FromAccountId, m.ToAccountId, m.AmountMinor)
	var out *depositsv1.WithdrawResponse
	err = pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		if r, err := replay(ctx, tx, key, h); err != nil || r != nil {
			out = r
			return err
		}
		p, err := s.owned(ctx, tx, m.PartyId, m.FromAccountId, true)
		if err != nil {
			return err
		}
		bal, err := s.balances(ctx, []string{m.FromAccountId})
		if err != nil {
			return connect.NewError(connect.CodeUnavailable, err)
		}
		var under int64
		if err := tx.QueryRow(ctx, `select coalesce(sum(amount_minor), 0) from notice where account_id = $1 and status = 'PENDING'`,
			m.FromAccountId).Scan(&under); err != nil {
			return err
		}
		if amount > bal[m.FromAccountId]-under {
			return connect.NewError(connect.CodeFailedPrecondition, errors.New("the amount is more than is available: money already under notice is set aside"))
		}
		if p.NoticeDays == 0 {
			if err := s.transfer(ctx, key, m.FromAccountId, m.ToAccountId, amount, codeInternal, m.Actor, "withdrawal"); err != nil {
				return err
			}
			out = &depositsv1.WithdrawResponse{State: "MOVED"}
			_, err := tx.Exec(ctx, `insert into move (idempotency_key, request_hash, state) values ($1, $2, 'MOVED')`, key, h)
			return err
		}
		id, err := uuid.NewV5(namespace, key)
		if err != nil {
			return err
		}
		now := s.now().UTC()
		due := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, p.NoticeDays)
		if _, err := tx.Exec(ctx, `insert into notice (notice_id, account_id, party_id, to_account_id, amount_minor, given_at, due_on, status, idempotency_key)
			values ($1, $2, $3, $4, $5, $6, $7, 'PENDING', $8)`, id, m.FromAccountId, m.PartyId, m.ToAccountId, amount, now, due, key); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `insert into move (idempotency_key, request_hash, state, notice_id) values ($1, $2, 'NOTICE_GIVEN', $3)`, key, h, id); err != nil {
			return err
		}
		out = &depositsv1.WithdrawResponse{State: "NOTICE_GIVEN", Notice: &depositsv1.Notice{
			NoticeId: id, AmountMinor: m.AmountMinor, ToAccountId: m.ToAccountId, GivenAt: now.Format(time.RFC3339),
			DueOn: due.Format("2006-01-02"), Status: "PENDING",
		}}
		return nil
	})
	if err != nil {
		return nil, internal(err)
	}
	return connect.NewResponse(out), nil
}

// CancelNotice implements the handler.
func (s *Service) CancelNotice(ctx context.Context, req *connect.Request[depositsv1.CancelNoticeRequest]) (*connect.Response[depositsv1.CancelNoticeResponse], error) {
	m := req.Msg
	if m.PartyId == "" || m.NoticeId == "" {
		return nil, invalid("party_id and notice_id are required")
	}
	var n depositsv1.Notice
	var amount int64
	var given, due time.Time
	err := s.Pool.QueryRow(ctx, `update notice set status = 'CANCELLED' where notice_id = $1 and party_id = $2 and status = 'PENDING'
		returning notice_id, amount_minor, to_account_id, given_at, due_on, status`, m.NoticeId, m.PartyId).
		Scan(&n.NoticeId, &amount, &n.ToAccountId, &given, &due, &n.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no pending notice with that id"))
	}
	if err != nil {
		return nil, internal(err)
	}
	n.AmountMinor, n.GivenAt, n.DueOn = strconv.FormatInt(amount, 10), given.UTC().Format(time.RFC3339), due.Format("2006-01-02")
	return connect.NewResponse(&depositsv1.CancelNoticeResponse{Notice: &n}), nil
}
