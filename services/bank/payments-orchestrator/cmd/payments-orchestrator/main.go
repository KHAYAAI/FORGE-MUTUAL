// Command payments-orchestrator runs the domestic payment workflow worker and the payments API
// (services/bank/payments-orchestrator/SPEC.md).
package main

import (
	"bufio"
	"connectrpc.com/connect"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1/ledgerv1connect"
	"github.com/khayaai/forge-mutual/pkg/api/forge/party/v1/partyv1connect"
	"github.com/khayaai/forge-mutual/pkg/api/forge/payments/v1/paymentsv1connect"
	"github.com/khayaai/forge-mutual/pkg/api/forge/policy/v1/policyv1connect"
	"github.com/khayaai/forge-mutual/pkg/api/forge/rails/v1/railsv1connect"
	"github.com/khayaai/forge-mutual/pkg/api/forge/recon/v1/reconv1connect"
	"github.com/khayaai/forge-mutual/pkg/banklayer/bootstrap"
	"github.com/khayaai/forge-mutual/pkg/callerauth"
	"github.com/khayaai/forge-mutual/pkg/outbox"
	"github.com/khayaai/forge-mutual/pkg/screening"
	"github.com/khayaai/forge-mutual/pkg/serve"
	"github.com/khayaai/forge-mutual/pkg/systemaccounts"
	"github.com/khayaai/forge-mutual/services/bank/payments-orchestrator/internal/inbound"
	"github.com/khayaai/forge-mutual/services/bank/payments-orchestrator/internal/inflight"
	"github.com/khayaai/forge-mutual/services/bank/payments-orchestrator/internal/payments"
	"github.com/khayaai/forge-mutual/services/bank/payments-orchestrator/internal/service"
	"github.com/khayaai/forge-mutual/services/bank/payments-orchestrator/internal/store"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
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
	recon, err := reconFromEnv(outbound)
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

	ledger := ledgerv1connect.NewLedgerServiceClient(outbound, urls["LEDGER_URL"], callerAs("svc-payments"))
	settlement, err := payments.EnsureSettlementAccount(ctx, ledger, "BANK")
	if err != nil {
		return fmt.Errorf("settlement account: %w", err)
	}
	log.Info("settlement account ready", "account", settlement)
	sponsorPool, err := systemaccounts.Ensure(ctx, ledger, "BANK", "ZAR", 3003, "system:pool:ZAR:v1")
	if err != nil {
		return fmt.Errorf("sponsor pool account: %w", err)
	}

	hostPort := getenv("TEMPORAL_HOST", "localhost:7233")
	tc, err := client.Dial(client.Options{HostPort: hostPort, Namespace: getenv("TEMPORAL_NAMESPACE", "default")})
	if err != nil {
		return fmt.Errorf("temporal: %w", err)
	}
	defer tc.Close()
	taskQueue := getenv("TEMPORAL_TASK_QUEUE", "payments")

	acts := &payments.Activities{
		Store:       st,
		Party:       partyv1connect.NewPartyServiceClient(outbound, urls["PARTY_URL"], callerAs("svc-payments")),
		Policy:      policyv1connect.NewPolicyServiceClient(outbound, urls["POLICY_URL"], callerAs("svc-payments")),
		Ledger:      ledger,
		Rails:       railsv1connect.NewRailServiceClient(outbound, urls["RAILS_URL"], callerAs("svc-payments")),
		Screener:    scr,
		Recon:       recon,
		Settlement:  settlement,
		SponsorPool: sponsorPool,
		Entity:      "BANK",
	}
	w := worker.New(tc, taskQueue, worker.Options{})
	w.RegisterWorkflow(payments.DomesticPayment)
	w.RegisterActivity(acts)
	if err := w.Start(); err != nil {
		return fmt.Errorf("worker: %w", err)
	}
	defer w.Stop()

	go relay(ctx, st, pub, log)
	if os.Getenv("INBOUND_ENABLED") == "1" {
		if err := startInbound(ctx, log, st, ledger); err != nil {
			return err
		}
	}

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
		connect.WithInterceptors(callers.Interceptor(callerauth.HeaderSource())))
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	if err := inbound.Open(ctx, st.Pool()); err != nil {
		return err
	}
	inflightPath, inflightHandler := paymentsv1connect.NewInFlightServiceHandler(
		&inflight.Service{Pool: st.Pool(), Inbound: &inbound.Worker{Pool: st.Pool()}},
		connect.WithInterceptors(callers.Interceptor(callerauth.HeaderSource())))
	mux.Handle(inflightPath, inflightHandler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := st.Pool().Ping(r.Context()); err != nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	listen := getenv("PAYMENTS_LISTEN", ":8080")
	stopTracing, err := serve.InitTracing(ctx, "payments-orchestrator")
	if err != nil {
		return err
	}
	defer func() { _ = stopTracing(context.Background()) }()
	if err := serve.RefuseInProduction(serve.Switch{Name: "PAYMENTS_ALLOW_SIMULATION", Set: os.Getenv("PAYMENTS_ALLOW_SIMULATION") == "true"}); err != nil {
		return err
	}
	tlsCfg, err := serve.FromEnv(listen)
	if err != nil {
		return err
	}
	srv, err := serve.New(tlsCfg, mux, log)
	if err != nil {
		return err
	}
	log.Info("payments listening", "addr", listen, "task_queue", taskQueue)
	return serve.Run(ctx, srv, log)
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

// reconFromEnv connects to the reconciliation gate. There is no static fallback: a missing gate must never read as
// "no break".
func reconFromEnv(outbound *http.Client) (payments.ReconStatus, error) {
	url := os.Getenv("RECON_URL")
	if url == "" {
		return nil, errors.New("RECON_URL is required; payments reads the reconciliation gate (invariant 9)")
	}
	return payments.ConnectRecon{Client: reconv1connect.NewReconServiceClient(outbound, url, callerAs("svc-payments"))}, nil
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

// startInbound books the credits the bank reports, for every monitored asset, until the context ends. It is off unless
// INBOUND_ENABLED=1, so a deployment opts in explicitly. A failed pass is logged and retried on the next tick.
func startInbound(ctx context.Context, log *slog.Logger, st *store.Store, ledger ledgerv1connect.LedgerServiceClient) error {
	cfgPath := getenv("BANK_CONFIG", "config/banklayer.yaml")
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		return fmt.Errorf("inbound: bank config: %w", err)
	}
	cfg, err := bootstrap.Load(raw)
	if err != nil {
		return err
	}
	if err := serve.RefuseInProduction(serve.Switch{Name: "BANK_ALLOW_SIMULATION", Set: os.Getenv("BANK_ALLOW_SIMULATION") == "true"}); err != nil {
		return fmt.Errorf("bank layer: %w", err)
	}
	bank, err := bootstrap.Open(ctx, cfg, bootstrap.Options{Getenv: os.Getenv, AllowSimulation: os.Getenv("BANK_ALLOW_SIMULATION") == "true"})
	if err != nil {
		return fmt.Errorf("inbound: bank layer: %w", err)
	}
	if err := inbound.Open(ctx, st.Pool()); err != nil {
		return err
	}
	every, err := time.ParseDuration(getenv("INBOUND_INTERVAL", "1m"))
	if err != nil || every <= 0 {
		return errors.New("INBOUND_INTERVAL must be a positive duration")
	}
	w := &inbound.Worker{Pool: st.Pool(), Ledger: ledger, Bank: bank, Entity: "BANK", Now: time.Now}
	go func() {
		tick := time.NewTicker(every)
		defer tick.Stop()
		for {
			for _, asset := range []string{"ZAR", "USD"} {
				n, err := w.Ingest(ctx, asset, 48*time.Hour)
				if err != nil {
					log.Error("inbound pass failed", "asset", asset, "err", err)
					continue
				}
				if n > 0 {
					log.Info("inbound credits booked", "asset", asset, "count", n)
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
			}
		}
	}()
	log.Info("inbound credits enabled", "interval", every)
	return nil
}
