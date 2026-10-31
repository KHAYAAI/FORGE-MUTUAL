// Command reconciliation runs the sponsor and partner reconciliation loop and serves the gate that payments reads.
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
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1/ledgerv1connect"
	"github.com/khayaai/forge-mutual/pkg/api/forge/recon/v1/reconv1connect"
	"github.com/khayaai/forge-mutual/pkg/banklayer/bootstrap"
	"github.com/khayaai/forge-mutual/pkg/callerauth"
	"github.com/khayaai/forge-mutual/services/bank/reconciliation/internal/ledgerview"
	"github.com/khayaai/forge-mutual/services/bank/reconciliation/internal/recon"
	"github.com/khayaai/forge-mutual/services/bank/reconciliation/internal/runner"
	"github.com/khayaai/forge-mutual/services/bank/reconciliation/internal/service"
	"github.com/khayaai/forge-mutual/services/bank/reconciliation/internal/store"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("reconciliation stopped", "err", err)
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
	if ledgerURL == "" {
		return errors.New("LEDGER_URL is required")
	}
	callers, err := loadCallers(os.Getenv("RECON_CALLERS_FILE"))
	if err != nil {
		return err
	}
	interval, err := durationFromEnv("RECON_INTERVAL", 15*time.Minute)
	if err != nil {
		return err
	}
	maxAge, err := durationFromEnv("RECON_MAX_AGE", 30*time.Minute)
	if err != nil {
		return err
	}
	if maxAge <= interval {
		return errors.New("RECON_MAX_AGE must be longer than RECON_INTERVAL, or the gate closes between runs")
	}

	cfgPath := getenv("BANK_CONFIG", "config/banklayer.yaml")
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		return fmt.Errorf("bank config: %w", err)
	}
	cfg, err := bootstrap.Load(raw)
	if err != nil {
		return err
	}
	bank, err := bootstrap.Open(ctx, cfg, bootstrap.Options{Getenv: os.Getenv})
	if err != nil {
		return fmt.Errorf("bank layer: %w", err)
	}

	st, err := store.Open(ctx, dsn)
	if err != nil {
		return err
	}
	defer st.Close()

	ledger := ledgerv1connect.NewLedgerServiceClient(http.DefaultClient, ledgerURL, callerAs("svc-reconciliation"))
	rn := &runner.Runner{
		Ledger: ledgerview.Reader{Client: ledger, Entity: "BANK"},
		Bank:   bank,
		Store:  st,
		Now:    time.Now,
	}
	assets := make([]string, 0, len(recon.Rails))
	for _, r := range recon.Rails {
		assets = append(assets, r.Asset)
	}
	svc := &service.Service{Store: st, Runner: rn, Assets: assets, MaxAge: maxAge, Now: time.Now}

	go loop(ctx, log, rn, assets, interval)

	mux := http.NewServeMux()
	path, handler := reconv1connect.NewReconServiceHandler(svc,
		connect.WithInterceptors(callers.Interceptor(callerauth.DefaultHeader)))
	mux.Handle(path, handler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := st.Pool().Ping(r.Context()); err != nil {
			http.Error(w, "store unreachable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	srv := &http.Server{
		Addr:              getenv("RECON_ADDR", ":8093"),
		Handler:           h2c.NewHandler(mux, &http2.Server{}),
		ReadHeaderTimeout: 5 * time.Second,
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	log.Info("reconciliation listening", "addr", srv.Addr, "interval", interval, "max_age", maxAge)
	select {
	case <-ctx.Done():
	case err := <-errc:
		return err
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// loop runs every asset on the interval. A failed run is logged and retried on the next tick. The gate ages out
// meanwhile, so a failure cannot leave payments open.
func loop(ctx context.Context, log *slog.Logger, rn *runner.Runner, assets []string, every time.Duration) {
	tick := time.NewTicker(every)
	defer tick.Stop()
	for {
		for _, a := range assets {
			out, err := rn.Run(ctx, a)
			if err != nil {
				log.Error("reconciliation run failed", "asset", a, "err", err)
				continue
			}
			log.Info("reconciliation run", "asset", a, "outcome", out.Outcome, "all_matched", out.AllMatched)
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

func loadCallers(path string) (callerauth.Allowlist, error) {
	if path == "" {
		return nil, errors.New("RECON_CALLERS_FILE is required")
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

func callerAs(name string) connect.Option {
	return connect.WithInterceptors(connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			req.Header().Set(callerauth.DefaultHeader, name)
			return next(ctx, req)
		}
	}))
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

func getenv(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}
