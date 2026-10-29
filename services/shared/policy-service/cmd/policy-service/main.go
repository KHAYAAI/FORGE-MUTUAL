// Command policy-service answers policy decisions from a verified, signed bundle and audits every
// decision (services/shared/policy-service/SPEC.md).
package main

import (
	"context"
	"crypto/ed25519"
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
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/khayaai/forge-mutual/pkg/api/forge/policy/v1/policyv1connect"
	"github.com/khayaai/forge-mutual/pkg/callerauth"
	"github.com/khayaai/forge-mutual/pkg/outbox"
	"github.com/khayaai/forge-mutual/services/shared/policy-service/internal/bundle"
	"github.com/khayaai/forge-mutual/services/shared/policy-service/internal/engine"
	"github.com/khayaai/forge-mutual/services/shared/policy-service/internal/service"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("policy service stopped", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("DATABASE_URL is required")
	}
	callersFile := os.Getenv("POLICY_CALLERS_FILE")
	if callersFile == "" {
		return errors.New("POLICY_CALLERS_FILE is required")
	}
	allow, err := loadCallers(callersFile)
	if err != nil {
		return err
	}
	trusted, err := trustedKeys(os.Getenv("POLICY_TRUSTED_KEYS"))
	if err != nil {
		return err
	}
	devUnsigned := os.Getenv("POLICY_DEV_UNSIGNED") == "true"
	if devUnsigned && len(trusted) > 0 {
		return errors.New("POLICY_DEV_UNSIGNED cannot be combined with POLICY_TRUSTED_KEYS")
	}
	if devUnsigned {
		log.Warn("POLICY_DEV_UNSIGNED=true: bundle signature is NOT verified; development only")
	}
	b, err := bundle.Load(getenv("POLICY_BUNDLE_DIR", "policy"), trusted, devUnsigned)
	if err != nil {
		return fmt.Errorf("bundle: %w", err) // refuse to start on an unverified bundle
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := outbox.Migrate(ctx, pool); err != nil {
		return err
	}

	eng, err := engine.Compile(ctx, b)
	if err != nil {
		return err
	}
	pub, closePub, err := openPublisher(log)
	if err != nil {
		return err
	}
	defer closePub()
	go relay(ctx, pool, pub, log)

	svc := service.New(pool, eng)
	path, handler := policyv1connect.NewPolicyServiceHandler(svc,
		connect.WithInterceptors(allow.Interceptor(callerauth.DefaultHeader)))
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	listen := getenv("POLICY_LISTEN", ":8080")
	srv := &http.Server{Addr: listen, Handler: h2c.NewHandler(mux, &http2.Server{}), ReadHeaderTimeout: 5 * time.Second}
	errc := make(chan error, 1)
	go func() {
		log.Info("policy service listening", "addr", listen, "bundle_version", b.Version)
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

// relay publishes audit events until ctx is cancelled.
func relay(ctx context.Context, pool *pgxpool.Pool, pub outbox.Publisher, log *slog.Logger) {
	for {
		n, err := outbox.Relay(ctx, pool, 100, pub)
		switch {
		case ctx.Err() != nil:
			return
		case err != nil:
			log.Error("audit relay failed", "err", err)
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

func openPublisher(log *slog.Logger) (outbox.Publisher, func(), error) {
	switch getenv("POLICY_PUBLISHER", "kafka") {
	case "kafka":
		brokers := splitList(os.Getenv("KAFKA_BROKERS"))
		if len(brokers) == 0 {
			return nil, nil, errors.New("KAFKA_BROKERS is required when POLICY_PUBLISHER=kafka")
		}
		kp, err := outbox.NewKafkaPublisher(brokers)
		if err != nil {
			return nil, nil, err
		}
		return kp, kp.Close, nil
	case "log":
		log.Warn("POLICY_PUBLISHER=log: audit events are logged, not sent to Kafka; development only")
		return logPublisher{log}, func() {}, nil
	default:
		return nil, nil, errors.New("POLICY_PUBLISHER must be kafka or log")
	}
}

type logPublisher struct{ log *slog.Logger }

func (p logPublisher) Publish(_ context.Context, topic string, key, value []byte) error {
	p.log.Info("audit", "topic", topic, "key", string(key), "bytes", len(value))
	return nil
}

// trustedKeys parses the comma-separated base64 public keys.
func trustedKeys(s string) ([]ed25519.PublicKey, error) {
	var out []ed25519.PublicKey
	for _, part := range splitList(s) {
		k, err := bundle.ParseKey(part)
		if err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, nil
}

// loadCallers reads a JSON object mapping caller identity to RPC names.
func loadCallers(path string) (callerauth.Allowlist, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("callers file: %w", err)
	}
	var raw map[string][]string
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("callers file: %w", err)
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
