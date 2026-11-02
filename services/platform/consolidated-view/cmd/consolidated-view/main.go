// Command consolidated-view projects ledger transfers into the read model, reconciles it with the ledger on a schedule,
// and serves balances and activity to the workspace API.
package main

import (
	"connectrpc.com/connect"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/khayaai/forge-mutual/pkg/api/forge/consolidated/v1/consolidatedv1connect"
	"github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1/ledgerv1connect"
	"github.com/khayaai/forge-mutual/pkg/callerauth"
	"github.com/khayaai/forge-mutual/pkg/eventbus"
	"github.com/khayaai/forge-mutual/pkg/serve"
	"github.com/khayaai/forge-mutual/services/platform/consolidated-view/internal/projection"
	"github.com/khayaai/forge-mutual/services/platform/consolidated-view/internal/recon"
	"github.com/khayaai/forge-mutual/services/platform/consolidated-view/internal/service"
	"github.com/khayaai/forge-mutual/services/platform/consolidated-view/internal/store"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("consolidated-view stopped", "err", err)
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

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("DATABASE_URL is required")
	}
	brokers := splitList(os.Getenv("KAFKA_BROKERS"))
	if len(brokers) == 0 {
		return errors.New("KAFKA_BROKERS is required")
	}
	ledgerURL := os.Getenv("LEDGER_URL")
	if ledgerURL == "" {
		return errors.New("LEDGER_URL is required")
	}
	callers, err := loadCallers(os.Getenv("CONSOLIDATED_CALLERS_FILE"))
	if err != nil {
		return err
	}
	every, err := durationFromEnv("CONSOLIDATED_RECON_INTERVAL", 15*time.Minute)
	if err != nil {
		return err
	}
	entity := getenv("CONSOLIDATED_ENTITY", "BANK")

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := store.Open(ctx, pool); err != nil {
		return err
	}
	if err := eventbus.Migrate(ctx, pool); err != nil {
		return err
	}
	st := store.New(pool)

	consumer := &eventbus.Consumer{
		Name: "consolidated-view", Pool: pool, Brokers: brokers, Group: "consolidated-view",
		Topics: []string{projection.TopicTransferCommitted}, Handle: projection.Handle, Log: log,
	}
	go func() {
		if err := consumer.Run(ctx); err != nil {
			log.Error("projection consumer stopped", "err", err)
			stop()
		}
	}()

	rec := &recon.Reconciler{
		Ledger: ledgerv1connect.NewLedgerServiceClient(outbound, ledgerURL, callerAs("svc-consolidated-view")),
		Entity: entity, Store: st, Now: time.Now,
	}
	go reconLoop(ctx, log, rec, every)

	mux := http.NewServeMux()
	path, handler := consolidatedv1connect.NewConsolidatedViewServiceHandler(&service.Service{Store: st},
		connect.WithInterceptors(callers.Interceptor(callerauth.HeaderSource())))
	mux.Handle(path, handler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			http.Error(w, "store unreachable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	stopTracing, err := serve.InitTracing(ctx, "consolidated-view")
	if err != nil {
		return err
	}
	defer func() { _ = stopTracing(context.Background()) }()
	tlsCfg, err := serve.FromEnv(getenv("CONSOLIDATED_ADDR", ":8096"))
	if err != nil {
		return err
	}
	srv, err := serve.New(tlsCfg, mux, log)
	if err != nil {
		return err
	}
	log.Info("consolidated-view listening", "addr", srv.Addr)
	return serve.Run(ctx, srv, log)
}

// reconLoop corrects the read model on the schedule. A failed run is logged; the next tick tries again.
func reconLoop(ctx context.Context, log *slog.Logger, rec *recon.Reconciler, every time.Duration) {
	tick := time.NewTicker(every)
	defer tick.Stop()
	for {
		if res, err := rec.Run(ctx); err != nil {
			log.Error("reconciliation failed", "err", err)
		} else {
			log.Info("reconciled", "accounts", res.Accounts, "drifted", res.Drifted)
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
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
		return nil, errors.New("CONSOLIDATED_CALLERS_FILE is required")
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

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
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
