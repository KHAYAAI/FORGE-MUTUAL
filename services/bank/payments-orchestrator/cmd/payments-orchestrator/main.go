// Command payments-orchestrator runs the domestic payment workflow worker and the payments API
// (services/bank/payments-orchestrator/SPEC.md).
package main

import (
	"bufio"
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
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1/ledgerv1connect"
	"github.com/khayaai/forge-mutual/pkg/api/forge/party/v1/partyv1connect"
	"github.com/khayaai/forge-mutual/pkg/api/forge/payments/v1/paymentsv1connect"
	"github.com/khayaai/forge-mutual/pkg/api/forge/policy/v1/policyv1connect"
	"github.com/khayaai/forge-mutual/pkg/api/forge/rails/v1/railsv1connect"
	"github.com/khayaai/forge-mutual/pkg/callerauth"
	"github.com/khayaai/forge-mutual/pkg/outbox"
	"github.com/khayaai/forge-mutual/pkg/screening"
	"github.com/khayaai/forge-mutual/services/bank/payments-orchestrator/internal/payments"
	"github.com/khayaai/forge-mutual/services/bank/payments-orchestrator/internal/service"
	"github.com/khayaai/forge-mutual/services/bank/payments-orchestrator/internal/store"
)

func callerAs(name string) connect.Option {
	return connect.WithInterceptors(connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			req.Header().Set(callerauth.DefaultHeader, name)
			return next(ctx, req)
		}
	}))
}

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("payments orchestrator stopped", "err", err)
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
	urls := map[string]string{}
	for _, name := range []string{"PARTY_URL", "POLICY_URL", "LEDGER_URL", "RAILS_URL"} {
		v := os.Getenv(name)
		if v == "" {
			return fmt.Errorf("%s is required", name)
		}
		urls[name] = v
	}
	scr, err := screenerFromEnv()
	if err != nil {
		return err
	}
	recon, err := reconFromEnv()
	if err != nil {
		return err
	}
	callers, err := loadCallers(os.Getenv("PAYMENTS_CALLERS_FILE"))
	if err != nil {
		return err
	}
	pub, closePub, err := openPublisher(log)
	if err != nil {
		return err
	}
	defer closePub()

	st, err := store.Open(ctx, dsn)
	if err != nil {
		return err
	}
	defer st.Close()

	ledger := ledgerv1connect.NewLedgerServiceClient(http.DefaultClient, urls["LEDGER_URL"], callerAs("svc-payments"))
	settlement, err := payments.EnsureSettlementAccount(ctx, ledger, "BANK")
	if err != nil {
		return fmt.Errorf("settlement account: %w", err)
	}
	log.Info("settlement account ready", "account", settlement)

	hostPort := getenv("TEMPORAL_HOST", "localhost:7233")
	tc, err := client.Dial(client.Options{HostPort: hostPort, Namespace: getenv("TEMPORAL_NAMESPACE", "default")})
	if err != nil {
		return fmt.Errorf("temporal: %w", err)
	}
	defer tc.Close()
	taskQueue := getenv("TEMPORAL_TASK_QUEUE", "payments")

	acts := &payments.Activities{
		Store:      st,
		Party:      partyv1connect.NewPartyServiceClient(http.DefaultClient, urls["PARTY_URL"], callerAs("svc-payments")),
		Policy:     policyv1connect.NewPolicyServiceClient(http.DefaultClient, urls["POLICY_URL"], callerAs("svc-payments")),
		Ledger:     ledger,
		Rails:      railsv1connect.NewRailServiceClient(http.DefaultClient, urls["RAILS_URL"], callerAs("svc-payments")),
		Screener:   scr,
		Recon:      recon,
		Settlement: settlement,
		Entity:     "BANK",
	}
	w := worker.New(tc, taskQueue, worker.Options{})
	w.RegisterWorkflow(payments.DomesticPayment)
	w.RegisterActivity(acts)
	if err := w.Start(); err != nil {
		return fmt.Errorf("worker: %w", err)
	}
	defer w.Stop()

	go relay(ctx, st, pub, log)

	simulation := os.Getenv("PAYMENTS_ALLOW_SIMULATION") == "true"
	if simulation {
		log.Warn("PAYMENTS_ALLOW_SIMULATION=true: simulation scenarios are accepted; not for production")
	}
	svc := service.New(tc, service.Config{
		TaskQueue:       taskQueue,
		AllowSimulation: simulation,
		PollSeconds:     intFromEnv("PAYMENTS_POLL_SECONDS"),
		ApprovalSeconds: intFromEnv("PAYMENTS_APPROVAL_SECONDS"),
		MaxPolls:        int(intFromEnv("PAYMENTS_MAX_POLLS")),
	})
	path, handler := paymentsv1connect.NewPaymentsServiceHandler(svc,
		connect.WithInterceptors(callers.Interceptor(callerauth.DefaultHeader)))
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := st.Pool().Ping(r.Context()); err != nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	listen := getenv("PAYMENTS_LISTEN", ":8080")
	srv := &http.Server{Addr: listen, Handler: h2c.NewHandler(mux, &http2.Server{}), ReadHeaderTimeout: 5 * time.Second}
	errc := make(chan error, 1)
	go func() {
		log.Info("payments listening", "addr", listen, "task_queue", taskQueue)
		errc <- srv.ListenAndServe()
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdown)
}

func relay(ctx context.Context, pool *store.Store, pub outbox.Publisher, log *slog.Logger) {
	for {
		n, err := outbox.Relay(ctx, pool.Pool(), 100, pub)
		switch {
		case ctx.Err() != nil:
			return
		case err != nil:
			log.Error("event relay failed", "err", err)
			sleep(ctx, time.Second)
		case n == 0:
			sleep(ctx, 500*time.Millisecond)
		}
	}
}

func sleep(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}

// screenerFromEnv loads the beneficiary screening list. Without a list the service refuses to start: a
// payment must never pass screening on no list.
func screenerFromEnv() (screening.Screener, error) {
	path := os.Getenv("SCREENING_LIST_FILE")
	if path == "" {
		return nil, errors.New("SCREENING_LIST_FILE is required; payments never run without a screening list")
	}
	asOf, err := time.Parse(time.RFC3339, os.Getenv("SCREENING_LIST_AS_OF"))
	if err != nil {
		return nil, fmt.Errorf("SCREENING_LIST_AS_OF must be RFC 3339: %w", err)
	}
	maxAge, err := time.ParseDuration(getenv("SCREENING_MAX_AGE", "24h"))
	if err != nil {
		return nil, fmt.Errorf("SCREENING_MAX_AGE: %w", err)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var names []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if line := strings.TrimSpace(sc.Text()); line != "" && !strings.HasPrefix(line, "#") {
			names = append(names, line)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return screening.NewFixture(names, asOf, maxAge, time.Now)
}

// reconFromEnv reads the reconciliation status. It must be set explicitly, because the reconciliation
// service is not built yet and a missing value must never read as "no break".
func reconFromEnv() (payments.ReconStatus, error) {
	switch os.Getenv("RECON_BREAK_ACTIVE") {
	case "true":
		return payments.StaticRecon{Break: true}, nil
	case "false":
		return payments.StaticRecon{Break: false}, nil
	}
	return nil, errors.New("RECON_BREAK_ACTIVE must be set to true or false until the reconciliation service exists")
}

func openPublisher(log *slog.Logger) (outbox.Publisher, func(), error) {
	switch getenv("PAYMENTS_PUBLISHER", "kafka") {
	case "kafka":
		brokers := splitList(os.Getenv("KAFKA_BROKERS"))
		if len(brokers) == 0 {
			return nil, nil, errors.New("KAFKA_BROKERS is required when PAYMENTS_PUBLISHER=kafka")
		}
		kp, err := outbox.NewKafkaPublisher(brokers)
		if err != nil {
			return nil, nil, err
		}
		return kp, kp.Close, nil
	case "log":
		log.Warn("PAYMENTS_PUBLISHER=log: events are logged, not sent to Kafka; development only")
		return logPublisher{log}, func() {}, nil
	default:
		return nil, nil, errors.New("PAYMENTS_PUBLISHER must be kafka or log")
	}
}

type logPublisher struct{ log *slog.Logger }

func (p logPublisher) Publish(_ context.Context, topic string, key, value []byte) error {
	p.log.Info("event", "topic", topic, "key", string(key), "bytes", len(value))
	return nil
}

func loadCallers(path string) (callerauth.Allowlist, error) {
	if path == "" {
		return nil, errors.New("PAYMENTS_CALLERS_FILE is required")
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

// intFromEnv reads an optional non-negative integer; zero means the workflow default.
func intFromEnv(name string) int64 {
	v := os.Getenv(name)
	if v == "" {
		return 0
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
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
