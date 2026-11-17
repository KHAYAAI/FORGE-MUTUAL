// Package ledgerfake is an in-memory ledger for tests: it keeps balances, replays by idempotency key, and refuses a
// debit that would take a constrained account below zero, as TigerBeetle does.
package ledgerfake

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	"connectrpc.com/connect"

	ledgerv1 "github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1/ledgerv1connect"
)

// Ledger is the fake.
type Ledger struct {
	ledgerv1connect.UnimplementedLedgerServiceHandler
	mu          sync.Mutex
	seq         int
	accounts    map[string]string // idempotency key -> account
	constrained map[string]bool
	balance     map[string]int64
	transfers   map[string]bool
	Legs        []*ledgerv1.TransferLeg // every applied leg, in order
}

// New starts the fake behind a real ConnectRPC server and returns a client for it.
func New(t *testing.T) (*Ledger, ledgerv1connect.LedgerServiceClient) {
	f := &Ledger{accounts: map[string]string{}, constrained: map[string]bool{}, balance: map[string]int64{}, transfers: map[string]bool{}}
	mux := http.NewServeMux()
	path, h := ledgerv1connect.NewLedgerServiceHandler(f)
	mux.Handle(path, h)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return f, ledgerv1connect.NewLedgerServiceClient(srv.Client(), srv.URL)
}

// Seed funds an account from an outside "seed" account, so the books still sum to zero.
func (f *Ledger) Seed(account string, minor int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.balance[account] += minor
	f.balance["seed"] -= minor
}

// Balance returns an account's balance.
func (f *Ledger) Balance(account string) int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.balance[account]
}

// Open creates an account outside any service, e.g. a customer's current account.
func (f *Ledger) Open(constrained bool) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seq++
	id := fmt.Sprintf("%08d-0000-4000-8000-000000000000", f.seq)
	f.constrained[id] = constrained
	return id
}

func (f *Ledger) CreateAccounts(_ context.Context, r *connect.Request[ledgerv1.CreateAccountsRequest]) (*connect.Response[ledgerv1.CreateAccountsResponse], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if id, ok := f.accounts[r.Msg.IdempotencyKey]; ok {
		return connect.NewResponse(&ledgerv1.CreateAccountsResponse{Status: ledgerv1.Status_STATUS_REPLAYED, AccountIds: []string{id}}), nil
	}
	f.seq++
	id := fmt.Sprintf("%08d-0000-4000-8000-000000000000", f.seq)
	f.accounts[r.Msg.IdempotencyKey] = id
	f.constrained[id] = r.Msg.Accounts[0].DebitsMustNotExceedCredits
	return connect.NewResponse(&ledgerv1.CreateAccountsResponse{Status: ledgerv1.Status_STATUS_OK, AccountIds: []string{id}}), nil
}

func (f *Ledger) CreateTransfers(_ context.Context, r *connect.Request[ledgerv1.CreateTransfersRequest]) (*connect.Response[ledgerv1.CreateTransfersResponse], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := r.Msg.IdempotencyKey
	if f.transfers[key] {
		return connect.NewResponse(&ledgerv1.CreateTransfersResponse{Status: ledgerv1.Status_STATUS_REPLAYED}), nil
	}
	next := map[string]int64{}
	for k, v := range f.balance {
		next[k] = v
	}
	for _, l := range r.Msg.Legs {
		n, err := strconv.ParseInt(l.Amount.AmountMinor, 10, 64)
		if err != nil || n <= 0 {
			return connect.NewResponse(&ledgerv1.CreateTransfersResponse{Status: ledgerv1.Status_STATUS_REJECTED, RejectionCodes: []string{"AMOUNT"}}), nil
		}
		next[l.DebitAccountId] -= n
		next[l.CreditAccountId] += n
		if f.constrained[l.DebitAccountId] && next[l.DebitAccountId] < 0 {
			return connect.NewResponse(&ledgerv1.CreateTransfersResponse{Status: ledgerv1.Status_STATUS_REJECTED, RejectionCodes: []string{"INSUFFICIENT_FUNDS"}}), nil
		}
	}
	f.balance = next
	f.transfers[key] = true
	f.Legs = append(f.Legs, r.Msg.Legs...)
	return connect.NewResponse(&ledgerv1.CreateTransfersResponse{Status: ledgerv1.Status_STATUS_OK}), nil
}

func (f *Ledger) GetBalances(_ context.Context, r *connect.Request[ledgerv1.GetBalancesRequest]) (*connect.Response[ledgerv1.GetBalancesResponse], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := &ledgerv1.GetBalancesResponse{}
	for _, id := range r.Msg.AccountIds {
		out.Balances = append(out.Balances, &ledgerv1.AccountBalance{AccountId: id, Asset: "ZAR", AvailableMinor: strconv.FormatInt(f.balance[id], 10)})
	}
	return connect.NewResponse(out), nil
}

// Sum is the total of every balance: double entry keeps it at zero.
func (f *Ledger) Sum() int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	var s int64
	for _, v := range f.balance {
		s += v
	}
	return s
}
