// Command deposits-service runs savings and notice deposit accounts: it serves the API and does the daily accrual,
// notice payouts and monthly interest.
//
// Configuration:
//
//	DATABASE_URL              Postgres (required)
//	LEDGER_URL                ledger-gateway-bank (required)
//	CONSOLIDATED_URL          consolidated-view, for which accounts a party owns (required)
//	DEPOSITS_CALLERS_FILE     caller allow-list (required)
//	DEPOSITS_ADDR             listen address, default :8110
//	DEPOSITS_RATES            optional "savings=650,notice-32=775,notice-90=850" (basis points; placeholders, VERIFY)
//	DEPOSITS_WORK_INTERVAL    how often the daily work is tried, default 15m
//	INSURED_DEPOSITS          "true" only when the sponsor confirms cover
//	FORGE_APPROVAL_SAVINGS, FORGE_APPROVAL_NOTICE_DEPOSITS   production approvals (pkg/licence)
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
	"strconv"
	"strings"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	consolidatedv1 "github.com/khayaai/forge-mutual/pkg/api/forge/consolidated/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/consolidated/v1/consolidatedv1connect"
	"github.com/khayaai/forge-mutual/pkg/api/forge/deposits/v1/depositsv1connect"
	"github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1/ledgerv1connect"
	"github.com/khayaai/forge-mutual/pkg/callerauth"
	"github.com/khayaai/forge-mutual/pkg/licence"
	"github.com/khayaai/forge-mutual/pkg/serve"
	"github.com/khayaai/forge-mutual/services/bank/deposits-service/internal/deposits"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("deposits-service stopped", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	outbound, err := serve.OutboundFromEnv()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	dsn, ledgerURL, consolidatedURL := os.Getenv("DATABASE_URL"), os.Getenv("LEDGER_URL"), os.Getenv("CONSOLIDATED_URL")
	if dsn == "" || ledgerURL == "" || consolidatedURL == "" {
		return errors.New("DATABASE_URL, LEDGER_URL and CONSOLIDATED_URL are required")
	}
	callers, err := loadCallers(os.Getenv("DEPOSITS_CALLERS_FILE"))
	if err != nil {
		return err
	}
	gate, err := licence.FromEnv(os.Getenv)
	if err != nil {
		return err
	}
	products, err := withRates(deposits.DefaultProducts(), os.Getenv("DEPOSITS_RATES"))
	if err != nil {
		return err
	}
	every, err := time.ParseDuration(getenv("DEPOSITS_WORK_INTERVAL", "15m"))
	if err != nil || every <= 0 {
		return errors.New("DEPOSITS_WORK_INTERVAL must be a positive duration")
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := deposits.Open(ctx, pool); err != nil {
		return err
	}
	ledger := ledgerv1connect.NewLedgerServiceClient(outbound, ledgerURL, callerAs("svc-deposits"))
	svc := &deposits.Service{
		Pool: pool, Ledger: ledger, Products: products, Licence: gate, Entity: "BANK",
		Insured: os.Getenv("INSURED_DEPOSITS") == "true",
		Owners:  owners{consolidatedv1connect.NewConsolidatedViewServiceClient(outbound, consolidatedURL, callerAs("svc-deposits"))},
	}
	if err := svc.EnsureSystemAccounts(ctx); err != nil {
		return fmt.Errorf("interest expense account: %w", err)
	}
	log.Warn("deposit rates are placeholders until treasury confirms them (docs/THRESHOLDS.md)", "products", len(products))

	go func() {
		tick := time.NewTicker(every)
		defer tick.Stop()
		for {
			if err := svc.RunDaily(ctx); err != nil {
				log.Error("daily deposit work failed; will retry", "err", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
			}
		}
	}()

	mux := http.NewServeMux()
	path, handler := depositsv1connect.NewDepositsServiceHandler(svc, connect.WithInterceptors(callers.Interceptor(callerauth.HeaderSource())))
	mux.Handle(path, handler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			http.Error(w, "store unreachable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	stopTracing, err := serve.InitTracing(ctx, "deposits-service")
	if err != nil {
		return err
	}
	defer func() { _ = stopTracing(context.Background()) }()
	tlsCfg, err := serve.FromEnv(getenv("DEPOSITS_ADDR", ":8110"))
	if err != nil {
		return err
	}
	srv, err := serve.New(tlsCfg, mux, log)
	if err != nil {
		return err
	}
	log.Info("deposits-service listening", "addr", srv.Addr, "licence", gate.Status())
	return serve.Run(ctx, srv, log)
}

// withRates applies "product=bps,..." over the defaults. An unknown product or a bad rate stops the service.
func withRates(ps []deposits.Product, spec string) ([]deposits.Product, error) {
	if strings.TrimSpace(spec) == "" {
		return ps, nil
	}
	for _, part := range strings.Split(spec, ",") {
		id, raw, ok := strings.Cut(strings.TrimSpace(part), "=")
		bps, err := strconv.ParseUint(raw, 10, 32)
		if !ok || err != nil || bps > 10000 {
			return nil, fmt.Errorf("DEPOSITS_RATES: %q is not product=basis-points", part)
		}
		found := false
		for i := range ps {
			if ps[i].ID == id {
				ps[i].RateBps, found = uint32(bps), true
			}
		}
		if !found {
			return nil, fmt.Errorf("DEPOSITS_RATES: unknown product %q", id)
		}
	}
	return ps, nil
}

type owners struct {
	client consolidatedv1connect.ConsolidatedViewServiceClient
}

func (o owners) Accounts(ctx context.Context, party string) (map[string]string, error) {
	res, err := o.client.GetBalances(ctx, connect.NewRequest(&consolidatedv1.GetBalancesRequest{PartyId: party}))
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
		return nil, errors.New("DEPOSITS_CALLERS_FILE is required")
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
			out[caller][m] = true
		}
	}
	return out, nil
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
