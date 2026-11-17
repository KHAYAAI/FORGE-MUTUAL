// Command acquiring-service runs payment links and point-of-sale payment requests: the merchant API, the payer's
// checkout, the acquirer's webhook, and next-day settlement.
//
// Configuration:
//
//	DATABASE_URL              Postgres (required)
//	LEDGER_URL                ledger-gateway-bank (required)
//	CONSOLIDATED_URL          consolidated-view, for which accounts a party owns (required)
//	ACQUIRING_CALLERS_FILE      caller allow-list (required)
//	ACQUIRING_ADDR            listen address, default :8130
//	ACQUIRING_WORK_INTERVAL   how often settlement is tried, default 15m
//	ACQUIRING_MDR_BPS         merchant discount rate, default 275 (a placeholder, docs/THRESHOLDS.md)
//	ACQUIRER                  "sim", the only acquirer built (VERIFY-ACQUIRER); refused in production
//	ACQUIRER_WEBHOOK_SECRET   the simulator's webhook secret (or _FILE)
//	PAY_BASE_URL              optional: the web app's origin for the simulator's checkout page (default: a relative path)
//	FORGE_APPROVAL_ACQUIRING  the production approval (pkg/licence)
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
	"syscall"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/khayaai/forge-mutual/pkg/api/forge/acquiring/v1/acquiringv1connect"
	consolidatedv1 "github.com/khayaai/forge-mutual/pkg/api/forge/consolidated/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/consolidated/v1/consolidatedv1connect"
	"github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1/ledgerv1connect"
	"github.com/khayaai/forge-mutual/pkg/callerauth"
	"github.com/khayaai/forge-mutual/pkg/licence"
	"github.com/khayaai/forge-mutual/pkg/serve"
	"github.com/khayaai/forge-mutual/services/bank/acquiring-service/internal/acquiring"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("acquiring-service stopped", "err", err)
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
	callers, err := loadCallers(os.Getenv("ACQUIRING_CALLERS_FILE"))
	if err != nil {
		return err
	}
	gate, err := licence.FromEnv(os.Getenv)
	if err != nil {
		return err
	}
	every, err := time.ParseDuration(getenv("ACQUIRING_WORK_INTERVAL", "15m"))
	if err != nil || every <= 0 {
		return errors.New("ACQUIRING_WORK_INTERVAL must be a positive duration")
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := acquiring.Open(ctx, pool); err != nil {
		return err
	}
	ledger := ledgerv1connect.NewLedgerServiceClient(outbound, ledgerURL, callerAs("svc-acquiring"))
	if os.Getenv("ACQUIRER") != "sim" {
		return errors.New("ACQUIRER must be \"sim\": no acquiring bank is chosen yet (VERIFY-ACQUIRER)")
	}
	if err := serve.RefuseInProduction(serve.Switch{Name: "ACQUIRER=sim", Set: true}); err != nil {
		return err
	}
	secret := os.Getenv("ACQUIRER_WEBHOOK_SECRET")
	if len(secret) < 16 {
		return errors.New("ACQUIRER_WEBHOOK_SECRET must be at least 16 characters")
	}
	mdr, err := strconv.ParseUint(getenv("ACQUIRING_MDR_BPS", "275"), 10, 32)
	if err != nil || mdr > 1000 {
		return errors.New("ACQUIRING_MDR_BPS must be 0 to 1000")
	}
	svc := &acquiring.Service{
		Pool: pool, Ledger: ledger, Licence: gate, Entity: "BANK", MDRBps: uint32(mdr), Log: log,
		Owners:   owners{consolidatedv1connect.NewConsolidatedViewServiceClient(outbound, consolidatedURL, callerAs("svc-acquiring"))},
		Acquirer: &acquiring.Simulator{PayBaseURL: os.Getenv("PAY_BASE_URL"), Secret: []byte(secret)},
	}
	if err := svc.EnsureSystemAccounts(ctx); err != nil {
		return fmt.Errorf("acquiring system accounts: %w", err)
	}
	log.Warn("the acquirer is the simulator and the merchant discount rate is a placeholder (VERIFY-ACQUIRER)", "mdr_bps", mdr)

	go func() {
		tick := time.NewTicker(every)
		defer tick.Stop()
		for {
			if n, err := svc.Settle(ctx); err != nil {
				log.Error("settlement failed; will retry", "err", err)
			} else if n > 0 {
				log.Info("settlements paid", "count", n)
			}
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
			}
		}
	}()

	mux := http.NewServeMux()
	path, handler := acquiringv1connect.NewAcquiringServiceHandler(svc, connect.WithInterceptors(callers.Interceptor(callerauth.HeaderSource())))
	mux.Handle(path, handler)
	coPath, coHandler := acquiringv1connect.NewCheckoutServiceHandler(svc, connect.WithInterceptors(callers.Interceptor(callerauth.HeaderSource())))
	mux.Handle(coPath, coHandler)
	// The acquirer's webhook authenticates itself by its signature, not by a caller certificate.
	mux.HandleFunc("POST /webhooks/acquirer", svc.Webhook)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			http.Error(w, "store unreachable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	stopTracing, err := serve.InitTracing(ctx, "acquiring-service")
	if err != nil {
		return err
	}
	defer func() { _ = stopTracing(context.Background()) }()
	tlsCfg, err := serve.FromEnv(getenv("ACQUIRING_ADDR", ":8130"))
	if err != nil {
		return err
	}
	srv, err := serve.New(tlsCfg, mux, log)
	if err != nil {
		return err
	}
	log.Info("acquiring-service listening", "addr", srv.Addr, "licence", gate.Status())
	return serve.Run(ctx, srv, log)
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
		return nil, errors.New("ACQUIRING_CALLERS_FILE is required")
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
