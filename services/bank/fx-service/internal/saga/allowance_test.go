package saga

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/khayaai/forge-mutual/services/bank/fx-service/internal/pricing"
	"github.com/khayaai/forge-mutual/services/bank/fx-service/internal/store"
)

// quote saves a bookable quote and returns its id.
func (r *rig) quote(party, sell, buy string, amount int64) string {
	r.t.Helper()
	p, err := pricing.Price(sell, buy, amount, mid, 50)
	if err != nil {
		r.t.Fatal(err)
	}
	id := fmt.Sprintf("q-%s-%d", party, time.Now().UnixNano())
	if err := pgx.BeginFunc(context.Background(), r.pool, func(tx pgx.Tx) error {
		return store.SaveQuote(context.Background(), tx, store.Quote{QuoteID: id, PartyID: party, SellAsset: sell, BuyAsset: buy,
			SellAmount: amount, BuyAmount: p.BuyAmount, Mid: mid, Rate: p.RateScaled, SpreadBps: 50, SpreadMinor: p.SpreadMinor, ExpiresAt: r.now.Add(time.Minute)})
	}); err != nil {
		r.t.Fatal(err)
	}
	return id
}

func (r *rig) lock(party, quote string, a *store.Allowance) error {
	_, err := store.LockQuote(context.Background(), r.pool, party, quote, "d-"+quote, "sell", "buy", r.now, a)
	return err
}

// An individual's rand sold for foreign currency in a year is capped. Selling foreign currency back does not count,
// and a voided deal gives its allowance back.
func TestAnIndividualsAnnualAllowanceIsEnforced(t *testing.T) {
	r := setup(t)
	limit := &store.Allowance{LimitMinor: 100_000}
	if err := r.lock("alw-p", r.quote("alw-p", "ZAR", "USD", 60_000), limit); err != nil {
		t.Fatal(err)
	}
	second := r.quote("alw-p", "ZAR", "USD", 50_000)
	if err := r.lock("alw-p", second, limit); !errors.Is(err, store.ErrAllowance) {
		t.Fatalf("over the allowance: %v", err)
	}
	if err := r.lock("alw-p", r.quote("alw-p", "USD", "ZAR", 1_000), limit); err != nil {
		t.Fatalf("selling foreign currency back: %v", err)
	}
	if _, err := r.pool.Exec(context.Background(), `update fx_deal set state = 'VOIDED' where party_id = 'alw-p' and sell_asset = 'ZAR'`); err != nil {
		t.Fatal(err)
	}
	if err := r.lock("alw-p", second, limit); err != nil {
		t.Fatalf("after a void gave the allowance back: %v", err)
	}
	if err := r.lock("alw-co", r.quote("alw-co", "ZAR", "USD", 900_000), nil); err != nil {
		t.Fatalf("a company has no allowance: %v", err)
	}
}

// Bookings racing for the last of an allowance cannot both win.
func TestConcurrentBookingsCannotExceedTheAllowance(t *testing.T) {
	r := setup(t)
	limit := &store.Allowance{LimitMinor: 100_000}
	quotes := make([]string, 5)
	for i := range quotes {
		quotes[i] = r.quote("alw-race", "ZAR", "USD", 30_000)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok := 0
	for _, q := range quotes {
		wg.Add(1)
		go func(q string) {
			defer wg.Done()
			if r.lock("alw-race", q, limit) == nil {
				mu.Lock()
				ok++
				mu.Unlock()
			}
		}(q)
	}
	wg.Wait()
	if ok != 3 {
		t.Fatalf("%d bookings of R300 against a R1 000 allowance; want 3", ok)
	}
}
