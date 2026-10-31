package inbound

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	ledgerv1 "github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1/ledgerv1connect"
	"github.com/khayaai/forge-mutual/pkg/banklayer"
	"github.com/khayaai/forge-mutual/pkg/banklayer/sim"
)

// fakeLedger keeps accounts by idempotency key and transfers by idempotency key, the way the real gateway replays.
type fakeLedger struct {
	ledgerv1connect.UnimplementedLedgerServiceHandler
	mu        sync.Mutex
	seq       int
	accounts  map[string]string
	transfers map[string][]*ledgerv1.TransferLeg
	failNext  bool
}

func newFake() *fakeLedger {
	return &fakeLedger{accounts: map[string]string{}, transfers: map[string][]*ledgerv1.TransferLeg{}}
}

func (f *fakeLedger) CreateAccounts(_ context.Context, req *connect.Request[ledgerv1.CreateAccountsRequest]) (*connect.Response[ledgerv1.CreateAccountsResponse], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := req.Msg.IdempotencyKey
	if id, ok := f.accounts[key]; ok {
		return connect.NewResponse(&ledgerv1.CreateAccountsResponse{Status: ledgerv1.Status_STATUS_REPLAYED, AccountIds: []string{id}}), nil
	}
	f.seq++
	id := fmt.Sprintf("%032x", f.seq)
	f.accounts[key] = id
	return connect.NewResponse(&ledgerv1.CreateAccountsResponse{Status: ledgerv1.Status_STATUS_OK, AccountIds: []string{id}}), nil
}

func (f *fakeLedger) CreateTransfers(_ context.Context, req *connect.Request[ledgerv1.CreateTransfersRequest]) (*connect.Response[ledgerv1.CreateTransfersResponse], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failNext {
		f.failNext = false
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("ledger down"))
	}
	key := req.Msg.IdempotencyKey
	if _, ok := f.transfers[key]; ok {
		return connect.NewResponse(&ledgerv1.CreateTransfersResponse{Status: ledgerv1.Status_STATUS_REPLAYED, TransferIds: []string{key}}), nil
	}
	f.transfers[key] = req.Msg.Legs
	return connect.NewResponse(&ledgerv1.CreateTransfersResponse{Status: ledgerv1.Status_STATUS_OK, TransferIds: []string{key}}), nil
}

func (f *fakeLedger) transferCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.transfers)
}

func (f *fakeLedger) account(key string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.accounts[key]
}

func setup(t *testing.T) (*Worker, *fakeLedger, *sim.Bank, *pgxpool.Pool) {
	t.Helper()
	dsn := os.Getenv("INBOUND_TEST_DSN")
	if dsn == "" {
		t.Skip("INBOUND_TEST_DSN not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := Open(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `truncate inbound_credit, inbound_reference`); err != nil {
		t.Fatal(err)
	}
	fl := newFake()
	mux := http.NewServeMux()
	path, h := ledgerv1connect.NewLedgerServiceHandler(fl)
	mux.Handle(path, h)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	bank := sim.New(nil)
	w := &Worker{
		Pool: pool, Ledger: ledgerv1connect.NewLedgerServiceClient(srv.Client(), srv.URL), Bank: bank,
		Entity: "BANK", Now: time.Now,
	}
	return w, fl, bank, pool
}

// A credit whose reference is registered is booked to that customer: debit the pool, credit the customer, code 10.
func TestMatchedCreditBooksToTheCustomer(t *testing.T) {
	ctx := context.Background()
	w, fl, bank, _ := setup(t)
	if err := w.RegisterReference(ctx, "FORGE-CUST-1", "ZAR", "cust-acc-1", "party-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := bank.InjectInbound("ZAR", 5000, "FORGE-CUST-1", false); err != nil {
		t.Fatal(err)
	}
	n, err := w.Ingest(ctx, "ZAR", time.Hour)
	if err != nil || n != 1 {
		t.Fatalf("ingest: %d, %v", n, err)
	}
	if fl.transferCount() != 1 {
		t.Fatalf("transfers = %d, want 1", fl.transferCount())
	}
	var leg *ledgerv1.TransferLeg
	for _, legs := range fl.transfers {
		leg = legs[0]
	}
	if leg.CreditAccountId != "cust-acc-1" || leg.DebitAccountId != fl.account("system:pool:ZAR:v1") ||
		leg.Amount.AmountMinor != "5000" || leg.Code != codeDepositIn {
		t.Fatalf("leg = %+v", leg)
	}
	if got, err := w.InFlight(ctx, "ZAR"); err != nil || got != 0 {
		t.Fatalf("a booked credit must leave nothing in flight: %d, %v", got, err)
	}
}

// An unmatched reference goes to suspense, where it waits for an operator. It is never dropped.
func TestUnmatchedCreditGoesToSuspense(t *testing.T) {
	ctx := context.Background()
	w, fl, bank, pool := setup(t)
	_, _ = bank.InjectInbound("ZAR", 700, "SOMEONE-ELSE", false)
	if _, err := w.Ingest(ctx, "ZAR", time.Hour); err != nil {
		t.Fatal(err)
	}
	var status, booked string
	if err := pool.QueryRow(ctx, `select status, booked_to from inbound_credit where reference = 'SOMEONE-ELSE'`).Scan(&status, &booked); err != nil {
		t.Fatal(err)
	}
	if status != StatusSuspense || booked != fl.account("system:suspense:ZAR:v1") {
		t.Fatalf("status %s booked to %s", status, booked)
	}
}

// Running ingest again must not post again: the bank still reports the credit, the key is the same.
func TestRepeatIngestBooksOnce(t *testing.T) {
	ctx := context.Background()
	w, fl, bank, _ := setup(t)
	_ = w.RegisterReference(ctx, "FORGE-CUST-2", "ZAR", "cust-acc-2", "party-2")
	_, _ = bank.InjectInbound("ZAR", 900, "FORGE-CUST-2", false)
	for i := 0; i < 3; i++ {
		if _, err := w.Ingest(ctx, "ZAR", time.Hour); err != nil {
			t.Fatal(err)
		}
	}
	if fl.transferCount() != 1 {
		t.Fatalf("three ingests posted %d transfers, want 1", fl.transferCount())
	}
}

// A crash after the credit was recorded but before it was posted must be finished by the next run.
func TestCrashAfterRecordingIsFinished(t *testing.T) {
	ctx := context.Background()
	w, fl, bank, pool := setup(t)
	_ = w.RegisterReference(ctx, "FORGE-CUST-3", "ZAR", "cust-acc-3", "party-3")
	m, _ := bank.InjectInbound("ZAR", 1200, "FORGE-CUST-3", false)
	if _, err := pool.Exec(ctx, `insert into inbound_credit (asset, movement_id, amount_minor, reference, status)
		values ('ZAR', $1, 1200, 'FORGE-CUST-3', 'PENDING')`, m.ID); err != nil {
		t.Fatal(err)
	}
	if n, err := w.Ingest(ctx, "ZAR", time.Hour); err != nil || n != 1 {
		t.Fatalf("recovery ingest: %d, %v", n, err)
	}
	if fl.transferCount() != 1 {
		t.Fatalf("recovery posted %d transfers", fl.transferCount())
	}
}

// While the ledger is down the credit stays PENDING, and it counts as in flight for reconciliation.
func TestLedgerOutageLeavesCreditInFlight(t *testing.T) {
	ctx := context.Background()
	w, fl, bank, _ := setup(t)
	_ = w.RegisterReference(ctx, "FORGE-CUST-4", "ZAR", "cust-acc-4", "party-4")
	_, _ = bank.InjectInbound("ZAR", 3300, "FORGE-CUST-4", false)
	fl.failNext = true
	if _, err := w.Ingest(ctx, "ZAR", time.Hour); err == nil {
		t.Fatal("a ledger outage must surface as an error")
	}
	got, err := w.InFlight(ctx, "ZAR")
	if err != nil || got != 3300 {
		t.Fatalf("in flight = %d, %v; want 3300", got, err)
	}
	if _, err := w.Ingest(ctx, "ZAR", time.Hour); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if got, _ := w.InFlight(ctx, "ZAR"); got != 0 {
		t.Fatalf("in flight after recovery = %d", got)
	}
}

// USD credits debit the partner mirror, not the sponsor pool.
func TestUSDCreditUsesThePartnerMirror(t *testing.T) {
	ctx := context.Background()
	w, fl, bank, _ := setup(t)
	_ = w.RegisterReference(ctx, "FORGE-USD-1", "USD", "usd-acc-1", "party-5")
	_, _ = bank.InjectInbound("USD", 700, "FORGE-USD-1", true)
	if n, err := w.Ingest(ctx, "USD", time.Hour); err != nil || n != 1 {
		t.Fatalf("usd ingest: %d, %v", n, err)
	}
	for _, legs := range fl.transfers {
		if legs[0].DebitAccountId != fl.account("system:pool:USD:v1") {
			t.Fatalf("USD credit debited %s, want the USD mirror", legs[0].DebitAccountId)
		}
		if legs[0].Amount.Asset != "USD" || legs[0].Amount.AmountMinor != strconv.Itoa(700) {
			t.Fatalf("leg = %+v", legs[0].Amount)
		}
	}
}

// A reference cannot be moved to a different account once bound, because that would misdirect later money.
func TestRebindingAReferenceIsRefused(t *testing.T) {
	ctx := context.Background()
	w, _, _, _ := setup(t)
	if err := w.RegisterReference(ctx, "FORGE-X", "ZAR", "acc-a", "party-a"); err != nil {
		t.Fatal(err)
	}
	if err := w.RegisterReference(ctx, "FORGE-X", "ZAR", "acc-a", "party-a"); err != nil {
		t.Fatalf("the same binding must be idempotent: %v", err)
	}
	if err := w.RegisterReference(ctx, "FORGE-X", "ZAR", "acc-b", "party-b"); err == nil {
		t.Fatal("rebinding to another account must be refused")
	}
}

var _ banklayer.BankLayer = (*sim.Bank)(nil)
