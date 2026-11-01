// Command fx-service quotes and books FX conversions, and resumes conversions a crash left part-way.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	consolidatedv1 "github.com/khayaai/forge-mutual/pkg/api/forge/consolidated/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/consolidated/v1/consolidatedv1connect"
	"github.com/khayaai/forge-mutual/pkg/api/forge/fx/v1/fxv1connect"
	"github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1/ledgerv1connect"
	"github.com/khayaai/forge-mutual/pkg/callerauth"
	"github.com/khayaai/forge-mutual/pkg/systemaccounts"
	"github.com/khayaai/forge-mutual/services/bank/fx-service/internal/rates"
	"github.com/khayaai/forge-mutual/services/bank/fx-service/internal/saga"
	"github.com/khayaai/forge-mutual/services/bank/fx-service/internal/service"
	"github.com/khayaai/forge-mutual/services/bank/fx-service/internal/store"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("fx-service stopped", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("DATABASE_URL is required")
	}
	ledgerURL := os.Getenv("LEDGER_URL")
	consolidatedURL := os.Getenv("CONSOLIDATED_URL")
	if ledgerURL == "" || consolidatedURL == "" {
		return errors.New("LEDGER_URL and CONSOLIDATED_URL are required")
	}
	callers, err := loadCallers(os.Getenv("FX_CALLERS_FILE"))
	if err != nil {
		return err
	}
	rate, err := rates.ParseScaled(os.Getenv("FX_RATE_ZAR_PER_USD"))
	if err != nil {
		return errors.New("FX_RATE_ZAR_PER_USD is required, as a decimal such as 18.25; the market-rate provider is not chosen yet (VERIFY)")
	}
	asOf, err := time.Parse(time.RFC3339, os.Getenv("FX_RATE_AS_OF"))
	if err != nil {
		return errors.New("FX_RATE_AS_OF is required, RFC 3339, the time the rate was observed")
	}
	log.Warn("the FX rate is a fixed value from configuration; a market-rate provider is required before live use (VERIFY)")
	maxAge, err := durationFromEnv("FX_RATE_MAX_AGE", time.Hour)
	if err != nil {
		return err
	}
	spread, err := int64FromEnv("FX_SPREAD_BPS", 50)
	if err != nil {
		return err
	}
	resumeEvery, err := durationFromEnv("FX_RESUME_INTERVAL", 30*time.Second)
	if err != nil {
		return err
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := store.Open(ctx, pool); err != nil {
		return err
	}

	ledger := ledgerv1connect.NewLedgerServiceClient(http.DefaultClient, ledgerURL, callerAs("svc-fx"))
	accounts := func(ctx context.Context, asset string) (string, string, error) {
		p, err := systemaccounts.Ensure(ctx, ledger, "BANK", asset, 2003, "system:fx-pool:"+asset+":v1")
		if err != nil {
			return "", "", err
		}
		f, err := systemaccounts.Ensure(ctx, ledger, "BANK", asset, 2002, "system:fx-fees:"+asset+":v1")
		return p, f, err
	}
	sg := &saga.Saga{Pool: pool, Ledger: ledger, Entity: "BANK", Accounts: accounts}
	owners := consolidatedOwners{client: consolidatedv1connect.NewConsolidatedViewServiceClient(http.DefaultClient, consolidatedURL, callerAs("svc-fx"))}

	go func() {
		tick := time.NewTicker(resumeEvery)
		defer tick.Stop()
		for {
			deals, err := store.OpenDeals(ctx, pool, time.Now().Add(-resumeEvery))
			if err != nil {
				log.Error("resume scan failed", "err", err)
			}
			for _, id := range deals {
				if state, err := sg.Run(ctx, id); err != nil {
					log.Error("resume failed; will retry", "deal", id, "state", state, "err", err)
				} else {
					log.Info("deal advanced", "deal", id, "state", state)
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
			}
		}
	}()

	svc := &service.Service{
		Pool: pool, Rates: rates.Static{Rate: rate, AsOf: asOf}, MaxAge: maxAge, SpreadBps: spread,
		Owners: owners, Saga: sg,
	}
	mux := http.NewServeMux()
	path, handler := fxv1connect.NewFXServiceHandler(svc, connect.WithInterceptors(callers.Interceptor(callerauth.DefaultHeader)))
	mux.Handle(path, handler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			http.Error(w, "store unreachable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	srv := &http.Server{Addr: getenv("FX_ADDR", ":8100"), Handler: h2c.NewHandler(mux, &http2.Server{}), ReadHeaderTimeout: 5 * time.Second}
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	log.Info("fx-service listening", "addr", srv.Addr, "spread_bps", spread)
	select {
	case <-ctx.Done():
	case err := <-errc:
		return err
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// consolidatedOwners asks the consolidated view which accounts a party owns.
type consolidatedOwners struct {
	client consolidatedv1connect.ConsolidatedViewServiceClient
}

func (c consolidatedOwners) Accounts(ctx context.Context, party string) (map[string]string, error) {
	res, err := c.client.GetBalances(ctx, connect.NewRequest(&consolidatedv1.GetBalancesRequest{PartyId: party}))
	if err != nil {
		return nil, fmt.Errorf("ownership: %w", err)
	}
	out := map[string]string{}
	for _, a := range res.Msg.Accounts {
		out[a.AccountId] = a.Asset
	}
	return out, nil
}

func callerAs(name string) connect.Option {
	return connect.WithInterceptors(connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			req.Header().Set(callerauth.DefaultHeader, name)
			return next(ctx, req)
		}
	}))
}

func loadCallers(path string) (callerauth.Allowlist, error) {
	if path == "" {
		return nil, errors.New("FX_CALLERS_FILE is required")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw map[string][]string
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, err
	}
	out := callerauth.Allowlist{}
	for caller, methods := range raw {
		out[caller] = map[string]bool{}
		for _, m := range methods {
			out[caller][strings.TrimSpace(m)] = true
		}
	}
	return out, nil
}

func durationFromEnv(name string, def time.Duration) (time.Duration, error) {
	v := os.Getenv(name)
	if v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", name)
	}
	return d, nil
}

func int64FromEnv(name string, def int64) (int64, error) {
	v := os.Getenv(name)
	if v == "" {
		return def, nil
	}
	var n int64
	if _, err := fmt.Sscan(v, &n); err != nil || n < 0 {
		return 0, fmt.Errorf("%s must be a non-negative integer", name)
	}
	return n, nil
}

func getenv(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}
