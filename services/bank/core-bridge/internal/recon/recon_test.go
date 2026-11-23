package recon

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	ledgerv1 "github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1/ledgerv1connect"
	"github.com/khayaai/forge-mutual/services/bank/core-bridge/internal/chart"
	"github.com/khayaai/forge-mutual/services/bank/core-bridge/internal/finfake"
	"github.com/khayaai/forge-mutual/services/bank/core-bridge/internal/store"
)

// book is the ledger for ZAR: accounts with codes, and posted balances.
type book struct {
	ledgerv1connect.UnimplementedLedgerServiceHandler
	code   map[string]uint32
	debit  map[string]int64
	credit map[string]int64
}

func (b *book) ListAccounts(_ context.Context, r *connect.Request[ledgerv1.ListAccountsRequest]) (*connect.Response[ledgerv1.ListAccountsResponse], error) {
	var ids []string
	for id, c := range b.code {
		if r.Msg.Ledger != 0 && r.Msg.Ledger != 710 {
			continue // every account in this book is on the ZAR ledger
		}
		if (r.Msg.Code == 0 || c == r.Msg.Code) && id > r.Msg.AfterAccountId {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	out := &ledgerv1.ListAccountsResponse{}
	for _, id := range ids {
		out.Accounts = append(out.Accounts, &ledgerv1.DirectoryAccount{AccountId: id, Ledger: 710, Code: b.code[id], Asset: "ZAR"})
	}
	return connect.NewResponse(out), nil
}

func (b *book) GetBalances(_ context.Context, r *connect.Request[ledgerv1.GetBalancesRequest]) (*connect.Response[ledgerv1.GetBalancesResponse], error) {
	out := &ledgerv1.GetBalancesResponse{}
	for _, id := range r.Msg.AccountIds {
		out.Balances = append(out.Balances, &ledgerv1.AccountBalance{AccountId: id,
			DebitsPosted: strconv.FormatInt(b.debit[id], 10), CreditsPosted: strconv.FormatInt(b.credit[id], 10),
			DebitsPending: "0", CreditsPending: "0"})
	}
	return connect.NewResponse(out), nil
}

const (
	acPool = "00000000000000000000000000000001"
	acCust = "00000000000000000000000000000002"
)

func setup(t *testing.T) (*Recon, *pgxpool.Pool, *book, *finfake.Fake) {
	t.Helper()
	dsn := os.Getenv("CORE_BRIDGE_TEST_DSN")
	if dsn == "" {
		t.Skip("CORE_BRIDGE_TEST_DSN not set")
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
	if _, err := pool.Exec(ctx, `truncate cb_journal, cb_recon_run`); err != nil {
		t.Fatal(err)
	}
	fake := finfake.New()
	fin := fake.Serve(t)
	gl := map[string]int64{}
	for _, asset := range chart.Assets {
		for _, e := range chart.Codes {
			kind := int(e.Kind)
			id, err := fin.CreateGLAccount(ctx, chart.GLCode(e.Code, asset), e.Name, kind)
			if err != nil {
				t.Fatal(err)
			}
			gl[chart.GLCode(e.Code, asset)] = id
		}
	}
	b := &book{code: map[string]uint32{acPool: 3003, acCust: 1001}, debit: map[string]int64{}, credit: map[string]int64{}}
	mux := http.NewServeMux()
	path, h := ledgerv1connect.NewLedgerServiceHandler(b)
	mux.Handle(path, h)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	r := &Recon{Pool: pool, Fin: fin, Ledger: ledgerv1connect.NewLedgerServiceClient(srv.Client(), srv.URL),
		Entity: "BANK", GL: gl}
	return r, pool, b, fake
}

// The same movement on both sides reconciles: no breaks across all fourteen GL accounts.
func TestMatchingBooksReconcile(t *testing.T) {
	r, _, b, fake := setup(t)
	ctx := context.Background()
	b.debit[acPool] = 15000
	b.credit[acCust] = 15000
	fin := r.Fin
	if _, err := fin.PostJournal(ctx, finJournal(t, r, 15000)); err != nil {
		t.Fatal(err)
	}
	_ = fake
	res, err := r.Run(ctx)
	if err != nil || res.Skipped != "" || res.Breaks != 0 || res.Checked != 14 {
		t.Fatalf("run: %+v, %v; want 14 checked, no breaks", res, err)
	}
}

// A ledger movement that never reached Fineract is a break, with its size.
func TestMissingPostingIsABreak(t *testing.T) {
	r, _, b, _ := setup(t)
	b.debit[acPool] = 15000
	b.credit[acCust] = 15500 // 500 more than Fineract has
	res, err := r.Run(context.Background())
	if err != nil || res.Breaks == 0 {
		t.Fatalf("run: %+v, %v; want breaks", res, err)
	}
	found := false
	for _, d := range res.Detail {
		if strings.Contains(d, "1001-ZAR") && strings.Contains(d, "difference") {
			found = true
		}
	}
	if !found {
		t.Fatalf("the break must name the account: %v", res.Detail)
	}
}

// Comparing while journals are queued would show breaks that are not real. The run is skipped and says why.
func TestRunIsSkippedWhileTheQueueDrains(t *testing.T) {
	r, pool, _, _ := setup(t)
	ctx := context.Background()
	if err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		return store.Queue(ctx, tx, "queued", "ZAR", 100, 3003, 1001, nowT(), store.StatusPending, "")
	}); err != nil {
		t.Fatal(err)
	}
	res, err := r.Run(ctx)
	if err != nil || res.Skipped == "" || res.Checked != 0 {
		t.Fatalf("run: %+v, %v; want a skip", res, err)
	}
}

func finJournal(t *testing.T, r *Recon, minor int64) fineractJournal {
	t.Helper()
	return fineractJournal{
		OfficeID: 1, Currency: "ZAR", Reference: "match-1",
		Debits:  []fineractLine{{GLAccountID: r.GL[chart.GLCode(3003, "ZAR")], AmountMinor: minor}},
		Credits: []fineractLine{{GLAccountID: r.GL[chart.GLCode(1001, "ZAR")], AmountMinor: minor}},
	}
}
