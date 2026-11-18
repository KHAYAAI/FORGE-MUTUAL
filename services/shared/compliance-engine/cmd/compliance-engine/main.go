// Command compliance-engine serves the compliance cases API. The monitoring rules have no event feed yet, so cases are
// opened by the store, not by this process.
package main

import (
	"connectrpc.com/connect"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/khayaai/forge-mutual/pkg/api/forge/cases/v1/casesv1connect"
	"github.com/khayaai/forge-mutual/pkg/api/forge/party/v1/partyv1connect"
	"github.com/khayaai/forge-mutual/pkg/callerauth"
	"github.com/khayaai/forge-mutual/pkg/eventbus"
	"github.com/khayaai/forge-mutual/pkg/screening"
	"github.com/khayaai/forge-mutual/pkg/serve"
	"github.com/khayaai/forge-mutual/services/shared/compliance-engine/internal/cases"
	"github.com/khayaai/forge-mutual/services/shared/compliance-engine/internal/feed"
	"github.com/khayaai/forge-mutual/services/shared/compliance-engine/internal/monitor"
	"github.com/khayaai/forge-mutual/services/shared/compliance-engine/internal/rescreen"
	"github.com/khayaai/forge-mutual/services/shared/compliance-engine/internal/service"
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
		log.Error("compliance-engine stopped", "err", err)
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
	callers, err := loadCallers(os.Getenv("CASES_CALLERS_FILE"))
	if err != nil {
		return err
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := cases.Open(ctx, pool); err != nil {
		return err
	}
	brokers := splitList(os.Getenv("KAFKA_BROKERS"))
	if len(brokers) == 0 {
		return errors.New("KAFKA_BROKERS is required: the monitoring rules read the event bus")
	}
	if err := eventbus.Migrate(ctx, pool); err != nil {
		return err
	}
	if err := feed.Open(ctx, pool); err != nil {
		return err
	}
	consumer := &eventbus.Consumer{
		Name: "compliance-engine", Pool: pool, Brokers: brokers, Group: "compliance-engine",
		Topics: []string{feed.TopicPaymentSettled, feed.TopicCreditBooked},
		Handle: (&feed.Feed{Cfg: monitor.DefaultConfig(), Now: time.Now}).Handle,
		Log:    log,
	}
	consumerErr := make(chan error, 1)
	go func() { consumerErr <- consumer.Run(ctx) }()

	// Sanctions re-screening of every customer, when the list changes and daily (FICA s.26A to s.26C). It runs only when
	// PARTY_URL and SCREENING_LIST_FILE are set, and production requires both.
	if err := startRescreen(ctx, log, pool); err != nil {
		return err
	}

	svc := &service.Service{Store: cases.New(pool), Now: time.Now}
	mux := http.NewServeMux()
	path, handler := casesv1connect.NewCasesServiceHandler(svc, connect.WithInterceptors(callers.Interceptor(callerauth.HeaderSource())))
	mux.Handle(path, handler)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			http.Error(w, "store unreachable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	stopTracing, err := serve.InitTracing(ctx, "compliance-engine")
	if err != nil {
		return err
	}
	defer func() { _ = stopTracing(context.Background()) }()
	tlsCfg, err := serve.FromEnv(getenv("CASES_ADDR", ":8400"))
	if err != nil {
		return err
	}
	srv, err := serve.New(tlsCfg, mux, log)
	if err != nil {
		return err
	}
	log.Info("compliance-engine listening", "addr", srv.Addr)
	return serve.Run(ctx, srv, log)
}

func loadCallers(path string) (callerauth.Allowlist, error) {
	if path == "" {
		return nil, errors.New("CASES_CALLERS_FILE is required")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw map[string][]string
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("CASES_CALLERS_FILE: %w", err)
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
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func getenv(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}

// startRescreen loads the list, then re-screens every customer now and at each RESCREEN_INTERVAL (default 1h; a run is
// repeated only for a new list version or a new day).
func startRescreen(ctx context.Context, log *slog.Logger, pool *pgxpool.Pool) error {
	partyURL, path := os.Getenv("PARTY_URL"), os.Getenv("SCREENING_LIST_FILE")
	if partyURL == "" || path == "" {
		if callerauth.IsProduction() {
			return errors.New("PARTY_URL and SCREENING_LIST_FILE are required in production: customers must be re-screened")
		}
		log.Warn("sanctions re-screening is off: PARTY_URL or SCREENING_LIST_FILE is not set")
		return nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	asOf, err := time.Parse(time.RFC3339, os.Getenv("SCREENING_LIST_AS_OF"))
	if err != nil {
		return fmt.Errorf("SCREENING_LIST_AS_OF must be RFC 3339: %w", err)
	}
	maxAge, err := time.ParseDuration(getenv("SCREENING_MAX_AGE", "24h"))
	if err != nil {
		return fmt.Errorf("SCREENING_MAX_AGE: %w", err)
	}
	var names []string
	for _, l := range strings.Split(string(raw), "\n") {
		if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") {
			names = append(names, l)
		}
	}
	scr, err := screening.NewFixture(names, asOf, maxAge, time.Now)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(raw)
	version := hex.EncodeToString(sum[:])
	every, err := time.ParseDuration(getenv("RESCREEN_INTERVAL", "1h"))
	if err != nil || every <= 0 {
		return errors.New("RESCREEN_INTERVAL must be a positive duration")
	}
	if err := rescreen.Open(ctx, pool); err != nil {
		return err
	}
	outbound, err := serve.OutboundFromEnv()
	if err != nil {
		return err
	}
	parties := partyv1connect.NewPartyServiceClient(outbound, partyURL, connect.WithInterceptors(connect.UnaryInterceptorFunc(
		func(next connect.UnaryFunc) connect.UnaryFunc {
			return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
				req.Header().Set(callerauth.DefaultHeader, "svc-compliance")
				return next(ctx, req)
			}
		})))
	r := &rescreen.Rescreener{Pool: pool, Parties: parties, Screener: scr}
	go func() {
		tick := time.NewTicker(every)
		defer tick.Stop()
		for {
			res, err := r.Run(ctx, version)
			switch {
			case err != nil:
				log.Error("sanctions re-screening failed; will retry", "err", err)
			case !res.Skipped:
				log.Info("sanctions re-screening done", "screened", res.Screened, "matched", res.Matched, "frozen", res.Frozen, "list", version[:12])
			}
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
			}
		}
	}()
	return nil
}
