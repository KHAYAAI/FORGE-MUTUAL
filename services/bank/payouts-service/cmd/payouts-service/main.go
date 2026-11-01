// Command payouts-service runs the payouts API and the Temporal worker that sends each batch's items through payments.
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
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	consolidatedv1 "github.com/khayaai/forge-mutual/pkg/api/forge/consolidated/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/consolidated/v1/consolidatedv1connect"
	"github.com/khayaai/forge-mutual/pkg/api/forge/payments/v1/paymentsv1connect"
	"github.com/khayaai/forge-mutual/pkg/api/forge/payouts/v1/payoutsv1connect"
	"github.com/khayaai/forge-mutual/pkg/callerauth"
	"github.com/khayaai/forge-mutual/services/bank/payouts-service/internal/service"
	"github.com/khayaai/forge-mutual/services/bank/payouts-service/internal/store"
	"github.com/khayaai/forge-mutual/services/bank/payouts-service/internal/workflow"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("payouts-service stopped", "err", err)
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
	paymentsURL := os.Getenv("PAYMENTS_URL")
	consolidatedURL := os.Getenv("CONSOLIDATED_URL")
	if paymentsURL == "" || consolidatedURL == "" {
		return errors.New("PAYMENTS_URL and CONSOLIDATED_URL are required")
	}
	callers, err := loadCallers(os.Getenv("PAYOUTS_CALLERS_FILE"))
	if err != nil {
		return err
	}
	pollEvery, err := durationFromEnv("PAYOUTS_POLL_INTERVAL", 30*time.Second)
	if err != nil {
		return err
	}
	maxPolls, err := intFromEnv("PAYOUTS_MAX_POLLS", 120)
	if err != nil {
		return err
	}
	concurrency, err := intFromEnv("PAYOUTS_CONCURRENCY", workflow.DefaultConcurrency)
	if err != nil {
		return err
	}
	maxItems, err := intFromEnv("PAYOUTS_MAX_ITEMS", 500)
	if err != nil {
		return err
	}
	maxItemMinor, err := int64FromEnv("PAYOUTS_MAX_ITEM_MINOR", 100_000_000)
	if err != nil {
		return err
	}
	taskQueue := getenv("TEMPORAL_TASK_QUEUE", "payouts")

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := store.Open(ctx, pool); err != nil {
		return err
	}

	tc, err := client.Dial(client.Options{
		HostPort: getenv("TEMPORAL_HOST", "localhost:7233"), Namespace: getenv("TEMPORAL_NAMESPACE", "default"),
	})
	if err != nil {
		return fmt.Errorf("temporal: %w", err)
	}
	defer tc.Close()

	payments := workflow.ConnectPayments{Client: paymentsv1connect.NewPaymentsServiceClient(http.DefaultClient, paymentsURL, callerAs("svc-payouts"))}
	acts := &workflow.Activities{Pool: pool, Payments: payments}

	w := worker.New(tc, taskQueue, worker.Options{})
	w.RegisterWorkflow(workflow.BulkPayout)
	w.RegisterWorkflow(workflow.PayoutItem)
	w.RegisterActivity(acts)
	if err := w.Start(); err != nil {
		return fmt.Errorf("temporal worker: %w", err)
	}
	defer w.Stop()

	owners := consolidatedOwners{client: consolidatedv1connect.NewConsolidatedViewServiceClient(http.DefaultClient, consolidatedURL, callerAs("svc-payouts"))}
	svc := &service.Service{
		Pool:   pool,
		Owners: owners,
		Runs: workflow.Starter{
			Client: tc, TaskQueue: taskQueue, Concurrency: concurrency, PollInterval: pollEvery, MaxPolls: maxPolls,
		},
		Limits: service.Limits{MaxItems: maxItems, MaxItemMinor: maxItemMinor},
	}

	mux := http.NewServeMux()
	path, handler := payoutsv1connect.NewPayoutsServiceHandler(svc, connect.WithInterceptors(callers.Interceptor(callerauth.DefaultHeader)))
	mux.Handle(path, handler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			http.Error(w, "store unreachable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	srv := &http.Server{
		Addr: getenv("PAYOUTS_ADDR", ":8200"), Handler: h2c.NewHandler(mux, &http2.Server{}), ReadHeaderTimeout: 5 * time.Second,
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	log.Info("payouts-service listening", "addr", srv.Addr, "task_queue", taskQueue)
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
		return nil, errors.New("PAYOUTS_CALLERS_FILE is required")
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

func intFromEnv(name string, def int) (int, error) {
	v := os.Getenv(name)
	if v == "" {
		return def, nil
	}
	var n int
	if _, err := fmt.Sscan(v, &n); err != nil || n <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return n, nil
}

func int64FromEnv(name string, def int64) (int64, error) {
	v := os.Getenv(name)
	if v == "" {
		return def, nil
	}
	var n int64
	if _, err := fmt.Sscan(v, &n); err != nil || n <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return n, nil
}

func getenv(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}
