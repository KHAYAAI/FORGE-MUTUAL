package runner

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"

	ledgerv1 "github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1/ledgerv1connect"
	"github.com/khayaai/forge-mutual/pkg/banklayer"
	"github.com/khayaai/forge-mutual/pkg/banklayer/sim"
	"github.com/khayaai/forge-mutual/services/bank/reconciliation/internal/ledgerview"
	"github.com/khayaai/forge-mutual/services/bank/reconciliation/internal/store"
)

// book is a fake ledger gateway for ZAR (ledger 710). Balances are posted by the test, the way the real ledger would
// post them for the same movements.
type book struct {
	mu       sync.Mutex
	codes    map[string]uint32
	debits   map[string]int64
	credits  map[string]int64
	ledgerID uint32
}

const (
	acPool     = "00000000000000000000000000000001" // 3003, sponsor pool mirror
	acCustomer = "00000000000000000000000000000002" // 1001
	acClearing = "00000000000000000000000000000003" // 3001
)

func newBook() *book {
	return &book{
		codes:    map[string]uint32{acPool: 3003, acCustomer: 1001, acClearing: 3001},
		debits:   map[string]int64{},
		credits:  map[string]int64{},
		ledgerID: 710,
	}
}

func (b *book) post(debit, credit string, amt int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.debits[debit] += amt
	b.credits[credit] += amt
}

func (b *book) ListAccounts(_ context.Context, req *connect.Request[ledgerv1.ListAccountsRequest]) (*connect.Response[ledgerv1.ListAccountsResponse], error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	var ids []string
	for id := range b.codes {
		if id > req.Msg.AfterAccountId {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	out := &ledgerv1.ListAccountsResponse{}
	for _, id := range ids {
		out.Accounts = append(out.Accounts, &ledgerv1.DirectoryAccount{
			AccountId: id, EntityCode: "BANK", Asset: "ZAR", Ledger: b.ledgerID, Code: b.codes[id],
		})
	}
	return connect.NewResponse(out), nil
}

func (b *book) GetBalances(_ context.Context, req *connect.Request[ledgerv1.GetBalancesRequest]) (*connect.Response[ledgerv1.GetBalancesResponse], error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := &ledgerv1.GetBalancesResponse{}
	for _, id := range req.Msg.AccountIds {
		out.Balances = append(out.Balances, &ledgerv1.AccountBalance{
			AccountId: id, DebitsPosted: strconv.FormatInt(b.debits[id], 10),
			CreditsPosted: strconv.FormatInt(b.credits[id], 10), DebitsPending: "0", CreditsPending: "0",
		})
	}
	return connect.NewResponse(out), nil
}

func serve(t *testing.T, b *book) ledgerv1connect.LedgerServiceClient {
	t.Helper()
	h := &handler{book: b}
	mux := http.NewServeMux()
	path, hh := ledgerv1connect.NewLedgerServiceHandler(h)
	mux.Handle(path, hh)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return ledgerv1connect.NewLedgerServiceClient(srv.Client(), srv.URL)
}

type handler struct {
	ledgerv1connect.UnimplementedLedgerServiceHandler
	book *book
}

func (h *handler) ListAccounts(ctx context.Context, r *connect.Request[ledgerv1.ListAccountsRequest]) (*connect.Response[ledgerv1.ListAccountsResponse], error) {
	return h.book.ListAccounts(ctx, r)
}

func (h *handler) GetBalances(ctx context.Context, r *connect.Request[ledgerv1.GetBalancesRequest]) (*connect.Response[ledgerv1.GetBalancesResponse], error) {
	return h.book.GetBalances(ctx, r)
}

// mutatingBank changes the book during every statement read, so no two snapshots agree.
type mutatingBank struct {
	banklayer.BankLayer
	hook func()
}

func (m mutatingBank) StatementSource() banklayer.StatementSource {
	return mutatingSource{m.BankLayer.StatementSource(), m.hook}
}

type mutatingSource struct {
	banklayer.StatementSource
	hook func()
}

func (m mutatingSource) Statement(ctx context.Context, asset string, date time.Time) (banklayer.Statement, error) {
	m.hook()
	return m.StatementSource.Statement(ctx, asset, date)
}

func openStore(t *testing.T) *store.Store {
	t.Helper()
	dsn := os.Getenv("RECON_TEST_DSN")
	if dsn == "" {
		t.Skip("RECON_TEST_DSN not set")
	}
	ctx := context.Background()
	s, err := store.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	if _, err := s.Pool().Exec(ctx, `truncate recon_investigation, recon_break, recon_run`); err != nil {
		t.Fatal(err)
	}
	return s
}

func gateOpen(t *testing.T, s *store.Store, at time.Time) (bool, []string) {
	t.Helper()
	g, err := s.Gate(context.Background(), []string{"ZAR"}, at, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return g.Open, g.Reasons
}

// The full story, mirrored between the simulator and the book: a credit arrives, a payment is initiated, the bank
// settles it before the ledger clears it (a real, transient break), and then the ledger catches up.
func TestRunFollowsTheBooksThroughAPaymentCycle(t *testing.T) {
	ctx := context.Background()
	st := openStore(t)
	b := newBook()
	bank := sim.New(nil)
	r := &Runner{Ledger: ledgerview.Reader{Client: serve(t, b), Entity: "BANK"}, Bank: bank, Store: st, Now: time.Now}

	// 1. A customer receives 100. The bank credits the pool; the ledger books the credit to the customer.
	if _, err := bank.InjectInbound("ZAR", 100, "FORGE-A", false); err != nil {
		t.Fatal(err)
	}
	b.post(acPool, acCustomer, 100)
	out, err := r.Run(ctx, "ZAR")
	if err != nil || out.Outcome != store.OutcomeConclusive || !out.AllMatched {
		t.Fatalf("step 1 must match: %+v, %v", out, err)
	}
	if ok, reasons := gateOpen(t, st, time.Now()); !ok {
		t.Fatalf("gate must open after a matched run: %v", reasons)
	}

	// 2. The customer pays 30. The ledger moves 30 from the customer to the clearing account; the bank still holds it.
	res, err := bank.Rails().Pay(ctx, banklayer.PayRequest{
		ID: "pay-1", Asset: "ZAR", AmountMinor: 30, Beneficiary: "Supplier", AccountNumber: "1234567890", BankCode: "250655",
	})
	if err != nil || res.Status != banklayer.StatusAccepted {
		t.Fatalf("pay: %+v, %v", res, err)
	}
	b.post(acCustomer, acClearing, 30)
	if out, _ := r.Run(ctx, "ZAR"); !out.AllMatched {
		t.Fatalf("step 2 must match while the bank still holds the money: %+v", out.Checks)
	}

	// 3. The bank settles the payment and debits the pool, but the ledger has not cleared it yet.
	for i := 0; i < 2; i++ {
		if _, err := bank.Rails().Status(ctx, "pay-1"); err != nil {
			t.Fatal(err)
		}
	}
	out, err = r.Run(ctx, "ZAR")
	if err != nil || out.AllMatched {
		t.Fatalf("an uncleared settlement must break the run: %+v, %v", out, err)
	}
	if ok, reasons := gateOpen(t, st, time.Now()); ok || !strings.Contains(strings.Join(reasons, " "), "sponsor_equation") {
		t.Fatalf("the break must close the gate and name the check: open=%v reasons=%v", ok, reasons)
	}

	// 4. The ledger clears the clearing account against the pool. The next run matches, and the gate reopens.
	b.post(acClearing, acPool, 30)
	out, err = r.Run(ctx, "ZAR")
	if err != nil || !out.AllMatched {
		t.Fatalf("after clearing the books must match again: %+v, %v", out.Checks, err)
	}
	if ok, reasons := gateOpen(t, st, time.Now()); !ok {
		t.Fatalf("the gate must reopen once the books match: %v", reasons)
	}
	open, _ := st.ListBreaks(ctx, true)
	if len(open) != 0 {
		t.Fatalf("no break may stay open after a matching run: %+v", open)
	}
}

// If the books move between the two snapshots around the statement read, the run cannot conclude. It must record an
// inconclusive run and change no break.
func TestUnstableBooksMakeTheRunInconclusive(t *testing.T) {
	ctx := context.Background()
	st := openStore(t)
	b := newBook()
	bank := sim.New(nil)
	_, _ = bank.InjectInbound("ZAR", 100, "FORGE-A", false)
	b.post(acPool, acCustomer, 100)
	churn := 0
	moving := mutatingBank{BankLayer: bank, hook: func() {
		churn++
		b.post(acPool, acCustomer, 1) // every statement read happens while the books move
	}}
	r := &Runner{Ledger: ledgerview.Reader{Client: serve(t, b), Entity: "BANK"}, Bank: moving, Store: st, Now: time.Now, Attempts: 3}

	out, err := r.Run(ctx, "ZAR")
	if err != nil {
		t.Fatal(err)
	}
	if out.Outcome != store.OutcomeInconclusive || out.AllMatched || len(out.Checks) != 0 {
		t.Fatalf("a run that never stabilises must be inconclusive with no checks: %+v", out)
	}
	if churn != 3 {
		t.Fatalf("statement read %d times, want 3 attempts", churn)
	}
	if ok, _ := gateOpen(t, st, time.Now()); ok {
		t.Fatal("an inconclusive run must not open the gate")
	}
	if all, _ := st.ListBreaks(ctx, false); len(all) != 0 {
		t.Fatalf("an inconclusive run must not record breaks: %+v", all)
	}
}

func TestUnknownAssetIsRefused(t *testing.T) {
	r := &Runner{}
	if _, err := r.Run(context.Background(), "EUR"); err == nil {
		t.Fatal("EUR is not monitored")
	}
}
