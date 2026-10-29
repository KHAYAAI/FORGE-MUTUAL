// Command ledger-gateway-bank is the only writer to the bank TigerBeetle cluster
// (services/bank/ledger-gateway-bank/SPEC.md).
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

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"connectrpc.com/connect"

	"github.com/khayaai/forge-mutual/services/bank/ledger-gateway-bank/gen/forge/ledger/v1/ledgerv1connect"
	"github.com/khayaai/forge-mutual/services/bank/ledger-gateway-bank/internal/auth"
	"github.com/khayaai/forge-mutual/services/bank/ledger-gateway-bank/internal/gateway"
	"github.com/khayaai/forge-mutual/services/bank/ledger-gateway-bank/internal/ledger"
	"github.com/khayaai/forge-mutual/services/bank/ledger-gateway-bank/internal/outbox"
	"github.com/khayaai/forge-mutual/services/bank/ledger-gateway-bank/internal/postgres"
)

type settings struct {
	listen        string
	store         string // "tigerbeetle" or "memory"
	tbClusterID   uint64
	tbAddresses   []string
	databaseURL   string
	kafkaBrokers  []string
	publisher     string // "kafka" or "log"
	callersFile   string
	relayInterval time.Duration
}

func loadSettings() (settings, error) {
	s := settings{
		listen:        getenv("LEDGER_LISTEN", ":8080"),
		store:         getenv("LEDGER_STORE", "tigerbeetle"),
		tbAddresses:   splitList(os.Getenv("TB_ADDRESSES")),
		databaseURL:   os.Getenv("DATABASE_URL"),
		kafkaBrokers:  splitList(os.Getenv("KAFKA_BROKERS")),
		publisher:     getenv("LEDGER_PUBLISHER", "kafka"),
		callersFile:   os.Getenv("LEDGER_CALLERS_FILE"),
		relayInterval: 500 * time.Millisecond,
	}
	if v := os.Getenv("TB_CLUSTER_ID"); v != "" {
		id, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return s, fmt.Errorf("TB_CLUSTER_ID: %w", err)
		}
		s.tbClusterID = id
	}
	if v := os.Getenv("LEDGER_RELAY_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return s, fmt.Errorf("LEDGER_RELAY_INTERVAL: %w", err)
		}
		s.relayInterval = d
	}
	if s.databaseURL == "" {
		return s, errors.New("DATABASE_URL is required")
	}
	if s.callersFile == "" {
		return s, errors.New("LEDGER_CALLERS_FILE is required")
	}
	if s.store == "tigerbeetle" && len(s.tbAddresses) == 0 {
		return s, errors.New("TB_ADDRESSES is required when LEDGER_STORE=tigerbeetle")
	}
	if s.publisher == "kafka" && len(s.kafkaBrokers) == 0 {
		return s, errors.New("KAFKA_BROKERS is required when LEDGER_PUBLISHER=kafka")
	}
	return s, nil
}

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("ledger gateway stopped", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	s, err := loadSettings()
	if err != nil {
		return err
	}
	allow, err := loadCallers(s.callersFile)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	repo, err := postgres.Connect(ctx, s.databaseURL)
	if err != nil {
		return err
	}
	defer repo.Close()

	store, closeStore, err := openStore(s, log)
	if err != nil {
		return err
	}
	defer closeStore()

	pub, closePub, err := openPublisher(s, log)
	if err != nil {
		return err
	}
	defer closePub()

	relay := &outbox.Relay{Repo: repo, Publisher: pub, BatchSize: 100, Interval: s.relayInterval, Log: log}
	go relay.Run(ctx)

	svc := gateway.NewService(gateway.DefaultConfig(), store, repo)
	path, handler := ledgerv1connect.NewLedgerServiceHandler(svc, connect.WithInterceptors(allow.Interceptor()))

	mux := http.NewServeMux()
	mux.Handle(path, handler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := repo.Ping(r.Context()); err != nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	// h2c serves HTTP/2 without TLS for traffic from the mesh sidecar, which terminates mTLS.
	srv := &http.Server{
		Addr:              s.listen,
		Handler:           h2c.NewHandler(mux, &http2.Server{}),
		ReadHeaderTimeout: 5 * time.Second,
	}
	errc := make(chan error, 1)
	go func() {
		log.Info("ledger gateway listening", "addr", s.listen, "store", s.store, "publisher", s.publisher)
		errc <- srv.ListenAndServe()
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

func openStore(s settings, log *slog.Logger) (ledger.Store, func(), error) {
	switch s.store {
	case "tigerbeetle":
		tb, err := ledger.NewTigerBeetle(s.tbClusterID, s.tbAddresses)
		if err != nil {
			return nil, nil, err
		}
		return tb, tb.Close, nil
	case "memory":
		log.Warn("LEDGER_STORE=memory: balances are not durable; development only")
		return ledger.NewMemStore(nil), func() {}, nil
	default:
		return nil, nil, fmt.Errorf("LEDGER_STORE must be tigerbeetle or memory, got %q", s.store)
	}
}

func openPublisher(s settings, log *slog.Logger) (outbox.Publisher, func(), error) {
	switch s.publisher {
	case "kafka":
		kp, err := outbox.NewKafkaPublisher(s.kafkaBrokers)
		if err != nil {
			return nil, nil, err
		}
		return kp, kp.Close, nil
	case "log":
		log.Warn("LEDGER_PUBLISHER=log: events are logged, not sent to Kafka; development only")
		return logPublisher{log}, func() {}, nil
	default:
		return nil, nil, fmt.Errorf("LEDGER_PUBLISHER must be kafka or log, got %q", s.publisher)
	}
}

type logPublisher struct{ log *slog.Logger }

func (p logPublisher) Publish(_ context.Context, topic string, key, value []byte) error {
	p.log.Info("event", "topic", topic, "key", string(key), "bytes", len(value))
	return nil
}

// loadCallers reads the caller allow-list: a JSON object mapping caller identity to RPC names.
func loadCallers(path string) (auth.Allowlist, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("callers file: %w", err)
	}
	var raw map[string][]string
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("callers file: %w", err)
	}
	out := auth.Allowlist{}
	for caller, methods := range raw {
		out[caller] = map[string]bool{}
		for _, m := range methods {
			out[caller][strings.TrimSpace(m)] = true
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

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
