// Command treasury-risk snapshots each monitored ledger, evaluates concentration, outflow, liquidity and suspense
// alerts, and writes alert transitions to the outbox.
package main

import (
	"connectrpc.com/connect"
	"context"
	"errors"
	"fmt"
	"github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1/ledgerv1connect"
	"github.com/khayaai/forge-mutual/pkg/callerauth"
	"github.com/khayaai/forge-mutual/pkg/serve"
	"github.com/khayaai/forge-mutual/services/bank/treasury-risk/internal/ledgerread"
	"github.com/khayaai/forge-mutual/services/bank/treasury-risk/internal/risk"
	"github.com/khayaai/forge-mutual/services/bank/treasury-risk/internal/store"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

// assets are the monitored ledgers: ZAR on the sponsor pool, USD on the partner mirror.
var assets = []ledgerread.Asset{
	{Code: "ZAR", Ledger: 710, PoolCode: 3003},
	{Code: "USD", Ledger: 840, PoolCode: 5001},
}

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("treasury-risk stopped", "err", err)
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
	ledgerURL := os.Getenv("LEDGER_URL")
	if ledgerURL == "" {
		return errors.New("LEDGER_URL is required")
	}
	interval, err := durationFromEnv("TREASURY_INTERVAL", 15*time.Minute)
	if err != nil {
		return err
	}
	th, err := thresholdsFromEnv()
	if err != nil {
		return err
	}
	st, err := store.Open(ctx, dsn)
	if err != nil {
		return err
	}
	defer st.Close()

	ledger := ledgerv1connect.NewLedgerServiceClient(outbound, ledgerURL, connect.WithInterceptors(
		connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
			return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
				req.Header().Set(callerauth.DefaultHeader, "svc-treasury-risk")
				return next(ctx, req)
			}
		})))
	reader := ledgerread.Reader{Client: ledger, Entity: getenv("TREASURY_ENTITY", "BANK"), Now: time.Now}

	log.Warn("alert events are written to the outbox; no relay publishes them until the broker is connected")
	go loop(ctx, log, st, reader, th, interval)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := st.Pool().Ping(r.Context()); err != nil {
			http.Error(w, "store unreachable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	stopTracing, err := serve.InitTracing(ctx, "treasury-risk")
	if err != nil {
		return err
	}
	defer func() { _ = stopTracing(context.Background()) }()
	tlsCfg, err := serve.FromEnv(getenv("TREASURY_ADDR", ":8095"))
	if err != nil {
		return err
	}
	srv, err := serve.New(tlsCfg, mux, log)
	if err != nil {
		return err
	}
	return serve.Run(ctx, srv, log)
}

// loop snapshots and evaluates every asset on the interval. A failed pass is logged; the next pass tries again.
func loop(ctx context.Context, log *slog.Logger, st *store.Store, r ledgerread.Reader, th risk.Thresholds, every time.Duration) {
	tick := time.NewTicker(every)
	defer tick.Stop()
	for {
		for _, a := range assets {
			if err := pass(ctx, st, r, a, th); err != nil {
				log.Error("treasury pass failed", "asset", a.Code, "err", err)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

func pass(ctx context.Context, st *store.Store, r ledgerread.Reader, a ledgerread.Asset, th risk.Thresholds) error {
	cur, err := r.Snapshot(ctx, a)
	if err != nil {
		return err
	}
	prev, err := st.Previous(ctx, a.Code, cur.At.Add(-24*time.Hour))
	if err != nil {
		return err
	}
	return st.Commit(ctx, cur, risk.Evaluate(cur, prev, th))
}

func thresholdsFromEnv() (risk.Thresholds, error) {
	th := risk.DefaultThresholds()
	var err error
	if th.ConcentrationBps, err = int64FromEnv("TREASURY_CONCENTRATION_BPS", th.ConcentrationBps); err != nil {
		return th, err
	}
	if th.OutflowBps, err = int64FromEnv("TREASURY_OUTFLOW_BPS", th.OutflowBps); err != nil {
		return th, err
	}
	if th.SuspenseMinor, err = int64FromEnv("TREASURY_SUSPENSE_MINOR", th.SuspenseMinor); err != nil {
		return th, err
	}
	return th, nil
}

func int64FromEnv(name string, def int64) (int64, error) {
	v := os.Getenv(name)
	if v == "" {
		return def, nil
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("%s must be a non-negative integer", name)
	}
	return n, nil
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
