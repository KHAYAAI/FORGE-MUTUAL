// Command lending-service runs credit facilities: overdrafts, working-capital loans and revenue-based financing. It
// serves the customer API and the credit desk, and does the daily work: instalments, overdraft interest and sweeps.
//
// Configuration:
//
//	DATABASE_URL              Postgres (required)
//	LEDGER_URL                ledger-gateway-bank (required)
//	CONSOLIDATED_URL          consolidated-view, for which accounts a party owns (required)
//	LENDING_CALLERS_FILE      caller allow-list (required)
//	LENDING_ADDR              listen address, default :8120
//	LENDING_WORK_INTERVAL     how often the daily work is tried, default 15m
//	FORGE_APPROVAL_OVERDRAFT, FORGE_APPROVAL_TERM_LOANS, FORGE_APPROVAL_REVENUE_BASED   production approvals (pkg/licence)
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

	consentv1 "github.com/khayaai/forge-mutual/pkg/api/forge/consent/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/consent/v1/consentv1connect"
	consolidatedv1 "github.com/khayaai/forge-mutual/pkg/api/forge/consolidated/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/consolidated/v1/consolidatedv1connect"
	"github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1/ledgerv1connect"
	"github.com/khayaai/forge-mutual/pkg/api/forge/lending/v1/lendingv1connect"
	"github.com/khayaai/forge-mutual/pkg/callerauth"
	"github.com/khayaai/forge-mutual/pkg/licence"
	"github.com/khayaai/forge-mutual/pkg/serve"
	"github.com/khayaai/forge-mutual/services/bank/lending-service/internal/lending"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("lending-service stopped", "err", err)
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
	callers, err := loadCallers(os.Getenv("LENDING_CALLERS_FILE"))
	if err != nil {
		return err
	}
	gate, err := licence.FromEnv(os.Getenv)
	if err != nil {
		return err
	}
	every, err := time.ParseDuration(getenv("LENDING_WORK_INTERVAL", "15m"))
	if err != nil || every <= 0 {
		return errors.New("LENDING_WORK_INTERVAL must be a positive duration")
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := lending.Open(ctx, pool); err != nil {
		return err
	}
	ledger := ledgerv1connect.NewLedgerServiceClient(outbound, ledgerURL, callerAs("svc-lending"))
	consolidated := consolidatedv1connect.NewConsolidatedViewServiceClient(outbound, consolidatedURL, callerAs("svc-lending"))
	svc := &lending.Service{
		Pool: pool, Ledger: ledger, Products: lending.DefaultProducts(), Licence: gate, Entity: "BANK",
		Owners: owners{consolidated}, Activity: consolidated,
		CreditProviderReg: os.Getenv("NCR_REGISTRATION"), RequirePreAgreement: os.Getenv("LENDING_REQUIRE_PRE_AGREEMENT") == "true" || callerauth.IsProduction(),
		RequireBureau: callerauth.IsProduction(),
	}
	if n, err := strconv.Atoi(getenv("LENDING_WRITE_OFF_AFTER_DAYS", "90")); err != nil || n < 30 || n > 365 {
		return errors.New("LENDING_WRITE_OFF_AFTER_DAYS must be 30 to 365")
	} else {
		svc.WriteOffAfterDays = n
	}
	// The credit bureau. No bureau is chosen yet (VERIFY-BUREAU): the simulator is the only one built, and production refuses it,
	// so production cannot assess credit until a real adapter exists (RequireBureau).
	switch os.Getenv("BUREAU") {
	case "sim":
		if err := serve.RefuseInProduction(serve.Switch{Name: "BUREAU=sim", Set: true}); err != nil {
			return err
		}
		consentURL := os.Getenv("CONSENT_URL")
		if consentURL == "" {
			return errors.New("CONSENT_URL is required with a credit bureau: an enquiry needs the customer's consent")
		}
		svc.Bureau = lending.SimBureau{}
		svc.Consents = consentCheck{consentv1connect.NewConsentServiceClient(outbound, consentURL, callerAs("svc-lending"))}
		log.Warn("the credit bureau is the simulator (VERIFY-BUREAU)")
	case "":
	default:
		return errors.New(`BUREAU must be "sim" or unset: no credit bureau is chosen yet`)
	}
	if err := svc.EnsureSystemAccounts(ctx); err != nil {
		return fmt.Errorf("interest income account: %w", err)
	}
	log.Warn("credit terms and the affordability rule are placeholders for the credit policy owner (docs/THRESHOLDS.md)")

	go func() {
		tick := time.NewTicker(every)
		defer tick.Stop()
		for {
			if err := svc.RunDaily(ctx); err != nil {
				log.Error("daily lending work failed; will retry", "err", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
			}
		}
	}()

	mux := http.NewServeMux()
	path, handler := lendingv1connect.NewLendingServiceHandler(svc, connect.WithInterceptors(callers.Interceptor(callerauth.HeaderSource())))
	mux.Handle(path, handler)
	deskPath, deskHandler := lendingv1connect.NewCreditDeskServiceHandler(svc, connect.WithInterceptors(callers.Interceptor(callerauth.HeaderSource())))
	mux.Handle(deskPath, deskHandler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			http.Error(w, "store unreachable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	stopTracing, err := serve.InitTracing(ctx, "lending-service")
	if err != nil {
		return err
	}
	defer func() { _ = stopTracing(context.Background()) }()
	tlsCfg, err := serve.FromEnv(getenv("LENDING_ADDR", ":8120"))
	if err != nil {
		return err
	}
	srv, err := serve.New(tlsCfg, mux, log)
	if err != nil {
		return err
	}
	log.Info("lending-service listening", "addr", srv.Addr, "licence", gate.Status())
	return serve.Run(ctx, srv, log)
}

type consentCheck struct {
	client consentv1connect.ConsentServiceClient
}

func (c consentCheck) Allowed(ctx context.Context, party, purpose, scope string) (bool, error) {
	res, err := c.client.Check(ctx, connect.NewRequest(&consentv1.CheckRequest{PartyId: party, Purpose: purpose, Scope: scope}))
	if err != nil {
		return false, fmt.Errorf("consent: %w", err)
	}
	return res.Msg.Allowed, nil
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
		return nil, errors.New("LENDING_CALLERS_FILE is required")
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
