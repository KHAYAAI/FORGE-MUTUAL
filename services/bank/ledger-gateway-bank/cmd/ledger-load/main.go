// Command ledger-load puts load on a running ledger gateway and then checks that no money was made, lost or moved
// twice.
//
// It opens a pool account and N customer accounts that must not go negative, funds each customer, and then runs W
// workers making random transfers between customers for D. Some transfers are bigger than the payer can afford, so
// refusals are part of the load. Some requests are sent twice with the same idempotency key, as a client does after
// a timeout, and the second answer must be a replay of the first with the same transfer IDs.
//
// Then it checks, from the ledger itself:
//   - every account holds exactly what the accepted transfers say it should;
//   - no customer account is negative;
//   - the balances of all the accounts sum to zero (every debit has its credit).
//
// It prints throughput and latency percentiles, and exits 1 if any check fails.
//
//	ledger-load -url http://127.0.0.1:8070 -accounts 50 -workers 32 -duration 60s
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	mrand "math/rand/v2"
	"net/http"
	"os"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"connectrpc.com/connect"

	ledgerv1 "github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1/ledgerv1connect"
)

const (
	entity  = "BANK"
	asset   = "ZAR"
	fund    = 1_000_000 // R10,000.00 per customer
	caller  = "svc-fx"  // allowed CreateAccounts, CreateTransfers and GetBalances on the stack
	codeIn  = 10        // DEPOSIT_IN
	codeXfr = 12        // INTERNAL_TRANSFER
)

func main() {
	url := flag.String("url", "http://127.0.0.1:8070", "ledger gateway")
	nAccounts := flag.Int("accounts", 50, "customer accounts")
	workers := flag.Int("workers", 32, "concurrent clients")
	duration := flag.Duration("duration", 60*time.Second, "how long to send transfers")
	replayEvery := flag.Int("replay-every", 10, "send one request in this many twice")
	flag.Parse()
	if err := run(*url, *nAccounts, *workers, *duration, *replayEvery); err != nil {
		fmt.Fprintln(os.Stderr, "ledger-load:", err)
		os.Exit(1)
	}
}

func uuid() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	h := hex.EncodeToString(b[:])
	return h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}

func withCaller[T any](m *T) *connect.Request[T] {
	r := connect.NewRequest(m)
	r.Header().Set("X-Forge-Caller", caller)
	return r
}

func run(url string, nAccounts, workers int, duration time.Duration, replayEvery int) error {
	hc := &http.Client{Timeout: 30 * time.Second, Transport: &http.Transport{MaxIdleConnsPerHost: workers * 2}}
	c := ledgerv1connect.NewLedgerServiceClient(hc, url)
	ctx := context.Background()
	run := uuid()[:8]

	specs := []*ledgerv1.AccountSpec{{Asset: asset, Code: 3003}}
	for i := 0; i < nAccounts; i++ {
		specs = append(specs, &ledgerv1.AccountSpec{Asset: asset, Code: 1001, DebitsMustNotExceedCredits: true, OwnerPartyId: uuid()})
	}
	acc, err := c.CreateAccounts(ctx, withCaller(&ledgerv1.CreateAccountsRequest{
		IdempotencyKey: "load-" + run + "-accounts", BusinessTxnId: uuid(), EntityCode: entity, Accounts: specs, Actor: "load",
	}))
	if err != nil || acc.Msg.Status != ledgerv1.Status_STATUS_OK {
		return fmt.Errorf("create accounts: %v %v", err, acc)
	}
	pool, customers := acc.Msg.AccountIds[0], acc.Msg.AccountIds[1:]
	expected := make(map[string]*atomic.Int64, len(customers)+1)
	for _, id := range acc.Msg.AccountIds {
		expected[id] = new(atomic.Int64)
	}
	for i, cust := range customers {
		r, err := c.CreateTransfers(ctx, withCaller(transfer("load-"+run+"-fund-"+strconv.Itoa(i), pool, cust, fund, codeIn)))
		if err != nil || r.Msg.Status != ledgerv1.Status_STATUS_OK {
			return fmt.Errorf("fund %d: %v %v", i, err, r)
		}
		expected[pool].Add(-fund)
		expected[cust].Add(fund)
	}
	fmt.Printf("opened 1 pool and %d customer accounts, R%d each\n", len(customers), fund/100)

	var (
		accepted, refused, replays, failures, retried atomic.Int64
		mu                                            sync.Mutex
		latencies                                     []time.Duration
		problems                                      []string
	)
	note := func(s string) {
		mu.Lock()
		if len(problems) < 20 {
			problems = append(problems, s)
		}
		mu.Unlock()
	}
	deadline := time.Now().Add(duration)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			rng := mrand.New(mrand.NewPCG(uint64(w), uint64(time.Now().UnixNano())))
			var mine []time.Duration
			for n := 0; time.Now().Before(deadline); n++ {
				from := customers[rng.IntN(len(customers))]
				to := customers[rng.IntN(len(customers))]
				if from == to {
					continue
				}
				amount := int64(1 + rng.IntN(20_000))
				if rng.IntN(20) == 0 {
					amount = fund * 3 // more than anyone has: must be refused
				}
				req := transfer(fmt.Sprintf("load-%s-w%d-%d", run, w, n), from, to, amount, codeXfr)
				start := time.Now()
				r, err := c.CreateTransfers(ctx, withCaller(req))
				// No answer (the gateway restarted, the connection dropped): send the same request again, as a real
				// caller must, until it is answered. Whether the first attempt was applied is for the ledger to say.
				for tries := 0; err != nil && connect.CodeOf(err) != connect.CodeInvalidArgument && tries < 120; tries++ {
					retried.Add(1)
					time.Sleep(500 * time.Millisecond)
					r, err = c.CreateTransfers(ctx, withCaller(req))
				}
				mine = append(mine, time.Since(start))
				if err != nil {
					failures.Add(1)
					note(fmt.Sprintf("transfer: %v", err))
					continue
				}
				switch r.Msg.Status {
				case ledgerv1.Status_STATUS_OK:
					accepted.Add(1)
					expected[from].Add(-amount)
					expected[to].Add(amount)
				case ledgerv1.Status_STATUS_REPLAYED:
					// The answer to an earlier attempt of this same request, whose reply was lost.
					if len(r.Msg.RejectionCodes) == 0 {
						accepted.Add(1)
						expected[from].Add(-amount)
						expected[to].Add(amount)
					} else {
						refused.Add(1)
					}
				case ledgerv1.Status_STATUS_REJECTED:
					refused.Add(1)
				default:
					failures.Add(1)
					note(fmt.Sprintf("a first send answered %v", r.Msg.Status))
				}
				if replayEvery > 0 && rng.IntN(replayEvery) == 0 {
					again, err := c.CreateTransfers(ctx, withCaller(req))
					for tries := 0; err != nil && tries < 120; tries++ {
						retried.Add(1)
						time.Sleep(500 * time.Millisecond)
						again, err = c.CreateTransfers(ctx, withCaller(req))
					}
					replays.Add(1)
					if err != nil || again.Msg.Status != ledgerv1.Status_STATUS_REPLAYED ||
						fmt.Sprint(again.Msg.TransferIds) != fmt.Sprint(r.Msg.TransferIds) ||
						fmt.Sprint(again.Msg.RejectionCodes) != fmt.Sprint(r.Msg.RejectionCodes) {
						failures.Add(1)
						note(fmt.Sprintf("replay of %s: %v %v, first answer %v", req.IdempotencyKey, err, again, r.Msg))
					}
				}
			}
			mu.Lock()
			latencies = append(latencies, mine...)
			mu.Unlock()
		}(w)
	}
	wg.Wait()

	total := accepted.Load() + refused.Load()
	secs := duration.Seconds()
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	pct := func(p float64) time.Duration {
		if len(latencies) == 0 {
			return 0
		}
		return latencies[int(p*float64(len(latencies)-1))]
	}
	fmt.Printf("%d transfer requests in %s with %d workers: %.0f a second\n", total, duration, workers, float64(total)/secs)
	fmt.Printf("  accepted %d, refused %d (insufficient funds), replays %d, retries after no answer %d, failures %d\n",
		accepted.Load(), refused.Load(), replays.Load(), retried.Load(), failures.Load())
	fmt.Printf("  latency p50 %s, p95 %s, p99 %s, max %s\n",
		pct(0.50).Round(time.Microsecond), pct(0.95).Round(time.Microsecond), pct(0.99).Round(time.Microsecond), pct(1).Round(time.Microsecond))

	// The checks, from the ledger itself.
	bal, err := c.GetBalances(ctx, withCaller(&ledgerv1.GetBalancesRequest{EntityCode: entity, AccountIds: acc.Msg.AccountIds}))
	if err != nil {
		return fmt.Errorf("balances: %w", err)
	}
	var sum int64
	wrong, negative := 0, 0
	for _, b := range bal.Msg.Balances {
		got, err := strconv.ParseInt(b.AvailableMinor, 10, 64)
		if err != nil {
			return err
		}
		sum += got
		if want := expected[b.AccountId].Load(); got != want {
			wrong++
			note(fmt.Sprintf("account %s holds %d, the accepted transfers say %d", b.AccountId, got, want))
		}
		if b.AccountId != pool && got < 0 {
			negative++
			note(fmt.Sprintf("customer account %s is negative: %d", b.AccountId, got))
		}
	}
	fmt.Printf("checks: %d accounts read; %d hold the wrong amount; %d customers negative; balances sum to %d\n",
		len(bal.Msg.Balances), wrong, negative, sum)
	for _, p := range problems {
		fmt.Println("  problem:", p)
	}
	if len(bal.Msg.Balances) != len(acc.Msg.AccountIds) || wrong > 0 || negative > 0 || sum != 0 || failures.Load() > 0 {
		return errors.New("an invariant does not hold")
	}
	if refused.Load() == 0 {
		return errors.New("no transfer was refused, so the overdraft check was not exercised")
	}
	fmt.Println("ledger-load: every invariant holds")
	return nil
}

func transfer(key, from, to string, amount int64, code uint32) *ledgerv1.CreateTransfersRequest {
	return &ledgerv1.CreateTransfersRequest{
		IdempotencyKey: key, BusinessTxnId: uuid(), EntityCode: entity, Actor: "load",
		Legs: []*ledgerv1.TransferLeg{{
			DebitAccountId: from, CreditAccountId: to, Code: code,
			Amount: &ledgerv1.Money{Asset: asset, AmountMinor: strconv.FormatInt(amount, 10)},
		}},
	}
}
