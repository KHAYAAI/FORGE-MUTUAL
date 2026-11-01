package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	fxv1 "github.com/khayaai/forge-mutual/pkg/api/forge/fx/v1"
	ledgerv1 "github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1/ledgerv1connect"
	"github.com/khayaai/forge-mutual/pkg/systemaccounts"
	"github.com/khayaai/forge-mutual/services/bank/fx-service/internal/ledgerfake"
	"github.com/khayaai/forge-mutual/services/bank/fx-service/internal/rates"
	"github.com/khayaai/forge-mutual/services/bank/fx-service/internal/saga"
	"github.com/khayaai/forge-mutual/services/bank/fx-service/internal/store"
)

// owners is a fake consolidated view: each party owns the accounts it is given.
type owners map[string]map[string]string

func (o owners) Accounts(_ context.Context, party string) (map[string]string, error) {
	return o[party], nil
}

// The customer quotes, books, and the conversion completes. Another party cannot book with these accounts.
func TestQuoteBookAndOwnership(t *testing.T) {
	dsn := os.Getenv("FX_TEST_DSN")
	if dsn == "" {
		t.Skip("FX_TEST_DSN not set")
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
	if _, err := pool.Exec(ctx, `truncate fx_deal, fx_quote`); err != nil {
		t.Fatal(err)
	}
	fake := ledgerfake.New()
	mux := http.NewServeMux()
	path, h := ledgerv1connect.NewLedgerServiceHandler(fake)
	mux.Handle(path, h)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	client := ledgerv1connect.NewLedgerServiceClient(srv.Client(), srv.URL)
	accounts := func(ctx context.Context, asset string) (string, string, error) {
		p, err := systemaccounts.Ensure(ctx, client, "BANK", asset, 2003, "system:fx-pool:"+asset+":v1")
		if err != nil {
			return "", "", err
		}
		f, err := systemaccounts.Ensure(ctx, client, "BANK", asset, 2002, "system:fx-fees:"+asset+":v1")
		return p, f, err
	}
	zarAcc := newAccount(t, client, fake, "ZAR", "svc-zar", 1_000_000)
	usdAcc := newAccount(t, client, fake, "USD", "svc-usd", 0)
	usdPool, _, _ := accounts(ctx, "USD")
	fake.Seed(usdPool, 100_000)

	now := time.Now()
	svc := &Service{
		Pool: pool, Rates: rates.Static{Rate: 1_825_000_000, AsOf: now}, MaxAge: time.Hour, SpreadBps: 50,
		Owners: owners{"party-1": {zarAcc: "ZAR", usdAcc: "USD"}},
		Saga:   &saga.Saga{Pool: pool, Ledger: client, Entity: "BANK", Accounts: accounts},
	}

	quote, err := svc.Quote(ctx, connect.NewRequest(&fxv1.QuoteRequest{PartyId: "party-1", SellAsset: "ZAR", BuyAsset: "USD", SellAmountMinor: "500000"}))
	if err != nil || quote.Msg.Disclosure == "" || quote.Msg.SpreadBps != 50 {
		t.Fatalf("quote: %+v, %v", quote, err)
	}

	// Another party tries to book the first party's accounts. It must be refused, and the quote must stay bookable.
	_, err = svc.Book(ctx, connect.NewRequest(&fxv1.BookRequest{PartyId: "party-2", QuoteId: quote.Msg.QuoteId,
		SellAccountId: zarAcc, BuyAccountId: usdAcc}))
	if connect.CodeOf(err) != connect.CodeNotFound && connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("another party booking: %v", err)
	}

	book, err := svc.Book(ctx, connect.NewRequest(&fxv1.BookRequest{PartyId: "party-1", QuoteId: quote.Msg.QuoteId,
		SellAccountId: zarAcc, BuyAccountId: usdAcc}))
	if err != nil || book.Msg.State != store.StateCompleted {
		t.Fatalf("book: %+v, %v", book, err)
	}
	deal, err := svc.GetDeal(ctx, connect.NewRequest(&fxv1.GetDealRequest{PartyId: "party-1", DealId: book.Msg.DealId}))
	if err != nil || deal.Msg.State != store.StateCompleted {
		t.Fatalf("deal: %+v, %v", deal, err)
	}

	// A quote can be booked once.
	if _, err := svc.Book(ctx, connect.NewRequest(&fxv1.BookRequest{PartyId: "party-1", QuoteId: quote.Msg.QuoteId,
		SellAccountId: zarAcc, BuyAccountId: usdAcc})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("second booking: %v", err)
	}
}

// A stale rate is not quoted on.
func TestStaleRateIsNotQuoted(t *testing.T) {
	dsn := os.Getenv("FX_TEST_DSN")
	if dsn == "" {
		t.Skip("FX_TEST_DSN not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	svc := &Service{Pool: pool, Rates: rates.Static{Rate: 1_825_000_000, AsOf: time.Now().Add(-3 * time.Hour)}, MaxAge: time.Hour, SpreadBps: 50}
	_, err = svc.Quote(context.Background(), connect.NewRequest(&fxv1.QuoteRequest{PartyId: "p", SellAsset: "ZAR", BuyAsset: "USD", SellAmountMinor: "100"}))
	if connect.CodeOf(err) != connect.CodeUnavailable {
		t.Fatalf("a stale rate must not be quoted: %v", err)
	}
}

func newAccount(t *testing.T, client ledgerv1connect.LedgerServiceClient, fake *ledgerfake.Fake, asset, key string, balance int64) string {
	t.Helper()
	res, err := client.CreateAccounts(context.Background(), connect.NewRequest(&ledgerv1.CreateAccountsRequest{
		IdempotencyKey: key, BusinessTxnId: key, EntityCode: "BANK",
		Accounts: []*ledgerv1.AccountSpec{{Asset: asset, Code: 1001}}, Actor: "test",
	}))
	if err != nil {
		t.Fatal(err)
	}
	id := res.Msg.AccountIds[0]
	fake.Seed(id, balance)
	return id
}
