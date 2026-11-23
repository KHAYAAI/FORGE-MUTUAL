package recon

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strconv"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	ledgerv1 "github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1/ledgerv1connect"
	"github.com/khayaai/forge-mutual/services/platform/consolidated-view/internal/store"
)

// book is the ledger: its accounts and posted balances. The test changes it to create drift.
type book struct {
	ledgerv1connect.UnimplementedLedgerServiceHandler
	accounts map[string]*ledgerv1.DirectoryAccount
	debit    map[string]int64
	credit   map[string]int64
}

func (b *book) ListAccounts(_ context.Context, r *connect.Request[ledgerv1.ListAccountsRequest]) (*connect.Response[ledgerv1.ListAccountsResponse], error) {
	var ids []string
	for id := range b.accounts {
		if id > r.Msg.AfterAccountId {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	out := &ledgerv1.ListAccountsResponse{}
	for _, id := range ids {
		out.Accounts = append(out.Accounts, b.accounts[id])
	}
	return connect.NewResponse(out), nil
}

func (b *book) GetBalances(_ context.Context, r *connect.Request[ledgerv1.GetBalancesRequest]) (*connect.Response[ledgerv1.GetBalancesResponse], error) {
	out := &ledgerv1.GetBalancesResponse{}
	for _, id := range r.Msg.AccountIds {
		out.Balances = append(out.Balances, &ledgerv1.AccountBalance{
			AccountId: id, DebitsPosted: strconv.FormatInt(b.debit[id], 10), CreditsPosted: strconv.FormatInt(b.credit[id], 10),
		})
	}
	return connect.NewResponse(out), nil
}

func setup(t *testing.T) (*Reconciler, *pgxpool.Pool, *book) {
	t.Helper()
	dsn := os.Getenv("CONSOLIDATED_TEST_DSN")
	if dsn == "" {
		t.Skip("CONSOLIDATED_TEST_DSN not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := store.Open(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `truncate cv_activity, cv_balance, cv_account, cv_recon_run`); err != nil {
		t.Fatal(err)
	}
	b := &book{accounts: map[string]*ledgerv1.DirectoryAccount{}, debit: map[string]int64{}, credit: map[string]int64{}}
	id := func(n int) string { return fmt.Sprintf("%032x", n) }
	b.accounts[id(1)] = &ledgerv1.DirectoryAccount{AccountId: id(1), Ledger: 710, Code: 1001, Asset: "ZAR", OwnerPartyId: "party-1", ProductId: "business-account", Insured: true}
	b.accounts[id(2)] = &ledgerv1.DirectoryAccount{AccountId: id(2), Ledger: 710, Code: 2001, Asset: "ZAR"}
	b.credit[id(1)] = 1500
	b.credit[id(2)] = 42

	mux := http.NewServeMux()
	path, h := ledgerv1connect.NewLedgerServiceHandler(b)
	mux.Handle(path, h)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	rec := &Reconciler{
		Ledger: ledgerv1connect.NewLedgerServiceClient(srv.Client(), srv.URL), Entity: "BANK",
		Store: store.New(pool), Now: func() time.Time { return time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC) },
	}
	return rec, pool, b
}

// Reconciliation names owners and insured status from the ledger, and corrects balances the projection got wrong.
// The projection had 1000 for the customer, where the ledger says 1500: drift. It had no row at all for the operating
// account, where the ledger holds 42: also drift, because a missing projection of money is still a miss.
func TestReconciliationClaimsOwnersAndCorrectsDrift(t *testing.T) {
	rec, pool, b := setup(t)
	ctx := context.Background()
	id := func(n int) string { return fmt.Sprintf("%032x", n) }
	if _, err := pool.Exec(ctx, `
		insert into cv_account (account_id, entity, asset) values ($1, 'BANK', 'ZAR')
		on conflict do nothing`, id(1)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `insert into cv_balance (account_id, debit_minor, credit_minor, source) values ($1, 0, 1000, 'EVENT')`, id(1)); err != nil {
		t.Fatal(err)
	}

	res, err := rec.Run(ctx)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Accounts != 2 || res.Drifted != 2 {
		t.Fatalf("result = %+v, want 2 accounts and 2 drifted", res)
	}
	var operating int64
	_ = pool.QueryRow(ctx, `select credit_minor from cv_balance where account_id = $1`, id(2)).Scan(&operating)
	if operating != 42 {
		t.Fatalf("the missed operating balance = %d after reconciliation, want 42", operating)
	}
	var owner string
	var insured bool
	_ = pool.QueryRow(ctx, `select owner_party_id, insured from cv_account where account_id = $1`, id(1)).Scan(&owner, &insured)
	if owner != "party-1" || !insured {
		t.Fatalf("owner = %q insured = %v", owner, insured)
	}
	var credit int64
	_ = pool.QueryRow(ctx, `select credit_minor from cv_balance where account_id = $1`, id(1)).Scan(&credit)
	if credit != b.credit[id(1)] {
		t.Fatalf("balance after reconciliation = %d, want the ledger's %d", credit, b.credit[id(1)])
	}

	// A second run with nothing changed finds no drift.
	if res, err := rec.Run(ctx); err != nil || res.Drifted != 0 {
		t.Fatalf("clean run: %+v, %v", res, err)
	}
}

// A ledger that cannot be read in full must record nothing: a partial run would look like a clean one.
func TestPartialLedgerReadRecordsNothing(t *testing.T) {
	rec, pool, _ := setup(t)
	ctx := context.Background()
	rec.Ledger = failingBalances{rec.Ledger}
	if _, err := rec.Run(ctx); err == nil {
		t.Fatal("a failed balance read must fail the run")
	}
	var n int
	_ = pool.QueryRow(ctx, `select count(*) from cv_recon_run`).Scan(&n)
	if n != 0 {
		t.Fatalf("a failed run recorded %d runs", n)
	}
}
