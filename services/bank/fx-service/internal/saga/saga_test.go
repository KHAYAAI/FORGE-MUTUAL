package saga

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	ledgerv1 "github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1/ledgerv1connect"
	"github.com/khayaai/forge-mutual/pkg/systemaccounts"
	"github.com/khayaai/forge-mutual/services/bank/fx-service/internal/ledgerfake"
	"github.com/khayaai/forge-mutual/services/bank/fx-service/internal/pricing"
	"github.com/khayaai/forge-mutual/services/bank/fx-service/internal/store"
)

const mid = 1825000000 // 18.25 ZAR per USD

type rig struct {
	t       *testing.T
	pool    *pgxpool.Pool
	fake    *ledgerfake.Fake
	client  ledgerv1connect.LedgerServiceClient
	saga    *Saga
	zarPool string
	zarFees string
	usdPool string
	usdFees string
	now     time.Time
}

// setup needs FX_TEST_DSN, a throwaway database. The fx tables are truncated.
func setup(t *testing.T) *rig {
	t.Helper()
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
	r := &rig{t: t, pool: pool, fake: fake, client: client, now: time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)}
	accounts := func(ctx context.Context, asset string) (string, string, error) {
		p, err := systemaccounts.Ensure(ctx, client, "BANK", asset, 2003, "system:fx-pool:"+asset+":v1")
		if err != nil {
			return "", "", err
		}
		f, err := systemaccounts.Ensure(ctx, client, "BANK", asset, 2002, "system:fx-fees:"+asset+":v1")
		return p, f, err
	}
	r.saga = &Saga{Pool: pool, Ledger: client, Entity: "BANK", Now: func() time.Time { return r.now }, Accounts: accounts}
	r.zarPool, r.zarFees, _ = accounts(ctx, "ZAR")
	r.usdPool, r.usdFees, _ = accounts(ctx, "USD")
	return r
}

func (r *rig) customer(asset string, key string, balance int64) string {
	r.t.Helper()
	res, err := r.client.CreateAccounts(context.Background(), connect.NewRequest(&ledgerv1.CreateAccountsRequest{
		IdempotencyKey: key, BusinessTxnId: key, EntityCode: "BANK",
		Accounts: []*ledgerv1.AccountSpec{{Asset: asset, Code: 1001}}, Actor: "test",
	}))
	if err != nil {
		r.t.Fatal(err)
	}
	id := res.Msg.AccountIds[0]
	r.fake.Seed(id, balance)
	return id
}

// book locks a quote for a conversion and returns the deal id.
func (r *rig) book(party string, sellAsset, buyAsset string, sellAmount int64, sellAcc, buyAcc string) string {
	r.t.Helper()
	p, err := pricing.Price(sellAsset, buyAsset, sellAmount, mid, 50)
	if err != nil {
		r.t.Fatal(err)
	}
	quoteID := fmt.Sprintf("q-%s-%d", party, time.Now().UnixNano())
	dealID := fmt.Sprintf("d-%s-%d", party, time.Now().UnixNano())
	err = pgx.BeginFunc(context.Background(), r.pool, func(tx pgx.Tx) error {
		return store.SaveQuote(context.Background(), tx, store.Quote{QuoteID: quoteID, PartyID: party, SellAsset: sellAsset,
			BuyAsset: buyAsset, SellAmount: sellAmount, BuyAmount: p.BuyAmount, Mid: mid, Rate: p.RateScaled, SpreadBps: 50,
			SpreadMinor: p.SpreadMinor, ExpiresAt: r.now.Add(time.Minute)})
	})
	if err != nil {
		r.t.Fatal(err)
	}
	if _, err := store.LockQuote(context.Background(), r.pool, party, quoteID, dealID, sellAcc, buyAcc, r.now); err != nil {
		r.t.Fatal(err)
	}
	return dealID
}

// Two workers must not drive one deal. While another session holds the deal's lock, Run refuses and the deal does not
// move; once the lock is released the deal completes.
func TestRunRefusesWhileAnotherWorkerHoldsTheDeal(t *testing.T) {
	r := setup(t)
	ctx := context.Background()
	sell := r.customer("ZAR", "cust-busy-zar", 1_000_000)
	buy := r.customer("USD", "cust-busy-usd", 0)
	r.fake.Seed(r.usdPool, 100_000)
	deal := r.book("busy", "ZAR", "USD", 100_000, sell, buy)

	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	key := "fx-deal:" + deal
	if _, err := conn.Exec(ctx, `select pg_advisory_lock(hashtextextended($1, 0))`, key); err != nil {
		t.Fatal(err)
	}

	if _, err := r.saga.Run(ctx, deal); !errors.Is(err, ErrBusy) {
		t.Fatalf("held deal: want ErrBusy, got %v", err)
	}
	d, err := store.GetDealByID(ctx, r.pool, deal)
	if err != nil || d.State != store.StateBooked {
		t.Fatalf("a refused run must leave the deal booked: err %v, state %+v", err, d)
	}

	if _, err := conn.Exec(ctx, `select pg_advisory_unlock(hashtextextended($1, 0))`, key); err != nil {
		t.Fatal(err)
	}
	state, err := r.saga.Run(ctx, deal)
	if err != nil || state != store.StateCompleted {
		t.Fatalf("after release: state %s, err %v", state, err)
	}
}

// The happy path moves each side exactly once. The customer's ZAR falls by the sell amount; the USD rises by the buy
// amount; the pool and fee accounts take the sell side and give the buy side. Every pending hold is released.
func TestConversionMovesEachSideOnce(t *testing.T) {
	r := setup(t)
	ctx := context.Background()
	zarAcc := r.customer("ZAR", "cust-zar-happy", 1_000_000)
	usdAcc := r.customer("USD", "cust-usd-happy", 0)
	r.fake.Seed(r.usdPool, 100_000)
	sell := int64(500_000)
	p, _ := pricing.Price("ZAR", "USD", sell, mid, 50)
	deal := r.book("party-h", "ZAR", "USD", sell, zarAcc, usdAcc)

	state, err := r.saga.Run(ctx, deal)
	if err != nil || state != store.StateCompleted {
		t.Fatalf("run: %s, %v", state, err)
	}
	if got := r.fake.Balance(zarAcc); got != 1_000_000-sell {
		t.Fatalf("customer ZAR = %d, want %d", got, 1_000_000-sell)
	}
	if got := r.fake.Balance(usdAcc); got != p.BuyAmount {
		t.Fatalf("customer USD = %d, want %d", got, p.BuyAmount)
	}
	if got := r.fake.Balance(r.zarFees); got != p.SpreadMinor {
		t.Fatalf("fee income = %d, want the spread %d", got, p.SpreadMinor)
	}
	if got := r.fake.Balance(r.zarPool); got != sell-p.SpreadMinor {
		t.Fatalf("ZAR pool = %d, want %d", got, sell-p.SpreadMinor)
	}
	if got := r.fake.Balance(r.usdPool); got != 100_000-p.BuyAmount {
		t.Fatalf("USD pool = %d, want %d", got, 100_000-p.BuyAmount)
	}
	if r.fake.PendingDebit(zarAcc) != 0 || r.fake.PendingDebit(r.usdPool) != 0 {
		t.Fatal("no hold may remain after completion")
	}
}

// If the buy cannot be funded, nothing moves and the sell hold is released.
func TestUnfundedBuyVoidsTheSellHold(t *testing.T) {
	r := setup(t)
	ctx := context.Background()
	zarAcc := r.customer("ZAR", "cust-zar-void", 1_000_000)
	usdAcc := r.customer("USD", "cust-usd-void", 0)
	deal := r.book("party-v", "ZAR", "USD", 500_000, zarAcc, usdAcc)

	state, err := r.saga.Run(ctx, deal)
	if err != nil || state != store.StateVoided {
		t.Fatalf("run: %s, %v; want VOIDED", state, err)
	}
	if got := r.fake.Balance(zarAcc); got != 1_000_000 {
		t.Fatalf("customer ZAR = %d after a voided deal, want 1000000", got)
	}
	if r.fake.PendingDebit(zarAcc) != 0 {
		t.Fatal("the sell hold must be released")
	}
}

// An uncertain failure while posting the buy leaves the deal where it was, and the next run finishes it.
func TestUncertainBuyPostResumes(t *testing.T) {
	r := setup(t)
	ctx := context.Background()
	zarAcc := r.customer("ZAR", "cust-zar-resume", 1_000_000)
	usdAcc := r.customer("USD", "cust-usd-resume", 0)
	r.fake.Seed(r.usdPool, 100_000)
	deal := r.book("party-r", "ZAR", "USD", 500_000, zarAcc, usdAcc)
	failed := false
	r.fake.Fail = func(op, key string) error {
		if op == "PostPending" && !failed && key == deal+":buy:post" {
			failed = true
			return connect.NewError(connect.CodeUnavailable, fmt.Errorf("timeout"))
		}
		return nil
	}
	if state, err := r.saga.Run(ctx, deal); err == nil || state == store.StateCompleted {
		t.Fatalf("the uncertain post must surface as an error and leave the deal open: %s, %v", state, err)
	}
	state, err := r.saga.Run(ctx, deal)
	if err != nil || state != store.StateCompleted {
		t.Fatalf("resume: %s, %v", state, err)
	}
	if got := r.fake.Balance(usdAcc); got == 0 {
		t.Fatal("the buy must have been delivered once the resume completed")
	}
}

// The sell is refused after the buy posted. The buy must be reversed, so the customer does not keep USD for a conversion
// that did not happen.
func TestRefusedSellReversesTheBuy(t *testing.T) {
	r := setup(t)
	ctx := context.Background()
	zarAcc := r.customer("ZAR", "cust-zar-rev", 1_000_000)
	usdAcc := r.customer("USD", "cust-usd-rev", 0)
	r.fake.Seed(r.usdPool, 100_000)
	deal := r.book("party-x", "ZAR", "USD", 500_000, zarAcc, usdAcc)
	r.fake.Fail = func(op, key string) error {
		if op == "PostPending" && key == deal+":sell:post" {
			return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("sell hold no longer valid"))
		}
		return nil
	}
	state, err := r.saga.Run(ctx, deal)
	if err != nil || state != store.StateReversed {
		t.Fatalf("run: %s, %v; want REVERSED", state, err)
	}
	if got := r.fake.Balance(usdAcc); got != 0 {
		t.Fatalf("customer kept %d USD after a reversed deal", got)
	}
	if got := r.fake.Balance(r.usdPool); got != 100_000 {
		t.Fatalf("USD pool = %d after reversal, want 100000", got)
	}
	if got := r.fake.Balance(zarAcc); got != 1_000_000 {
		t.Fatalf("customer ZAR = %d after reversal, want 1000000", got)
	}
}

// A crash after the buy posted and before the sell: the resume must not post the buy again, and must complete.
func TestCrashAfterBuyResumesWithoutRepeatingIt(t *testing.T) {
	r := setup(t)
	ctx := context.Background()
	zarAcc := r.customer("ZAR", "cust-zar-crash", 1_000_000)
	usdAcc := r.customer("USD", "cust-usd-crash", 0)
	r.fake.Seed(r.usdPool, 100_000)
	deal := r.book("party-c", "ZAR", "USD", 500_000, zarAcc, usdAcc)
	crashed := false
	r.fake.Fail = func(op, key string) error {
		if op == "PostPending" && !crashed && key == deal+":sell:post" {
			crashed = true
			return connect.NewError(connect.CodeUnavailable, fmt.Errorf("process died"))
		}
		return nil
	}
	if _, err := r.saga.Run(ctx, deal); err == nil {
		t.Fatal("the crash must surface")
	}
	d, _ := store.GetDealByID(ctx, r.pool, deal)
	if d.State != store.StateBuyPosted {
		t.Fatalf("state after crash = %s, want BUY_POSTED", d.State)
	}
	usdBefore := r.fake.Balance(usdAcc)
	state, err := r.saga.Run(ctx, deal)
	if err != nil || state != store.StateCompleted {
		t.Fatalf("resume: %s, %v", state, err)
	}
	if r.fake.Balance(usdAcc) != usdBefore {
		t.Fatalf("the resume changed the customer's USD: %d -> %d", usdBefore, r.fake.Balance(usdAcc))
	}
}

// A quote is locked at booking. Booking after it expires is refused, and a second booking of the same quote is refused.
func TestExpiredAndDoubleBookingsAreRefused(t *testing.T) {
	r := setup(t)
	ctx := context.Background()
	p, _ := pricing.Price("ZAR", "USD", 100_000, mid, 50)
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		return store.SaveQuote(ctx, tx, store.Quote{QuoteID: "q-exp", PartyID: "p", SellAsset: "ZAR", BuyAsset: "USD",
			SellAmount: 100_000, BuyAmount: p.BuyAmount, Mid: mid, Rate: p.RateScaled, SpreadBps: 50, SpreadMinor: p.SpreadMinor,
			ExpiresAt: r.now.Add(time.Minute)})
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.LockQuote(ctx, r.pool, "p", "q-exp", "d-exp", "a", "b", r.now.Add(2*time.Minute)); err != store.ErrNotBookable {
		t.Fatalf("an expired quote: %v, want ErrNotBookable", err)
	}
	if _, err := store.LockQuote(ctx, r.pool, "someone-else", "q-exp", "d-other", "a", "b", r.now); err != store.ErrNotFound {
		t.Fatalf("another party's quote: %v, want ErrNotFound", err)
	}
	if _, err := store.LockQuote(ctx, r.pool, "p", "q-exp", "d-1", "a", "b", r.now); err != nil {
		t.Fatalf("first booking: %v", err)
	}
	if _, err := store.LockQuote(ctx, r.pool, "p", "q-exp", "d-2", "a", "b", r.now); err != store.ErrNotBookable {
		t.Fatalf("second booking of the same quote: %v, want ErrNotBookable", err)
	}
}
