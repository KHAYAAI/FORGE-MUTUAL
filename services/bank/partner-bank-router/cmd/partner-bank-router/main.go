// Command partner-bank-router routes foreign-currency operations to partner banks by health and cost.
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
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/khayaai/forge-mutual/pkg/api/forge/partner/v1/partnerv1connect"
	"github.com/khayaai/forge-mutual/pkg/banklayer/bootstrap"
	"github.com/khayaai/forge-mutual/pkg/callerauth"
	"github.com/khayaai/forge-mutual/services/bank/partner-bank-router/internal/service"
	"github.com/khayaai/forge-mutual/services/bank/partner-bank-router/internal/store"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("partner-bank-router stopped", "err", err)
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
	callers, err := loadCallers(os.Getenv("PARTNER_CALLERS_FILE"))
	if err != nil {
		return err
	}
	costs, err := parseCosts(os.Getenv("PARTNER_COSTS_BPS"))
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(getenv("BANK_CONFIG", "config/banklayer.yaml"))
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

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := store.Open(ctx, pool); err != nil {
		return err
	}

	var partners []service.Partner
	for _, p := range bank.ForeignCurrency() {
		partners = append(partners, service.Partner{Provider: p, CostBps: costs[p.Partner()]})
	}
	if len(partners) == 0 {
		return errors.New("the bank layer offers no foreign-currency partner")
	}
	log.Info("partners configured", "count", len(partners))

	mux := http.NewServeMux()
	path, handler := partnerv1connect.NewPartnerRouterServiceHandler(&service.Service{Pool: pool, Partners: partners},
		connect.WithInterceptors(callers.Interceptor(callerauth.DefaultHeader)))
	mux.Handle(path, handler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			http.Error(w, "store unreachable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	srv := &http.Server{Addr: getenv("PARTNER_ROUTER_ADDR", ":8099"), Handler: h2c.NewHandler(mux, &http2.Server{}), ReadHeaderTimeout: 5 * time.Second}
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	select {
	case <-ctx.Done():
	case err := <-errc:
		return err
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// parseCosts reads "partner-a=10,partner-b=30" (basis points per operation).
func parseCosts(s string) (map[string]int64, error) {
	out := map[string]int64{}
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, v, ok := strings.Cut(part, "=")
		n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if !ok || err != nil || n < 0 {
			return nil, fmt.Errorf("PARTNER_COSTS_BPS entry %q must be name=non-negative-bps", part)
		}
		out[strings.TrimSpace(name)] = n
	}
	return out, nil
}

func loadCallers(path string) (callerauth.Allowlist, error) {
	if path == "" {
		return nil, errors.New("PARTNER_CALLERS_FILE is required")
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

func getenv(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}
