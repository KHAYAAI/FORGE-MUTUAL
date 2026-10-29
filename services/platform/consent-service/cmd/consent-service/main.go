// Command consent-service records what each party has agreed to, and answers whether a use is covered
// (services/platform/consent-service/SPEC.md).
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

	"github.com/khayaai/forge-mutual/pkg/api/forge/consent/v1/consentv1connect"
	"github.com/khayaai/forge-mutual/pkg/callerauth"
	"github.com/khayaai/forge-mutual/pkg/outbox"
	"github.com/khayaai/forge-mutual/services/platform/consent-service/internal/service"
	"github.com/khayaai/forge-mutual/services/platform/consent-service/internal/store"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("consent service stopped", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("DATABASE_URL is required")
	}
	callersFile := os.Getenv("CONSENT_CALLERS_FILE")
	if callersFile == "" {
		return errors.New("CONSENT_CALLERS_FILE is required")
	}
	allow, err := loadCallers(callersFile)
	if err != nil {
		return err
	}
	pub, closePub, err := openPublisher(log)
	if err != nil {
		return err
	}
	defer closePub()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	st, err := store.Open(ctx, dsn)
	if err != nil {
		return err
	}
	defer st.Close()
	go relay(ctx, st, pub, log)

	svc := service.New(st)
	path, handler := consentv1connect.NewConsentServiceHandler(svc,
		connect.WithInterceptors(allow.Interceptor(callerauth.DefaultHeader)))
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

	listen := getenv("CONSENT_LISTEN", ":8080")
	srv := &http.Server{Addr: listen, Handler: h2c.NewHandler(mux, &http2.Server{}), ReadHeaderTimeout: 5 * time.Second}
	errc := make(chan error, 1)
	go func() {
		log.Info("consent service listening", "addr", listen)
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

func relay(ctx context.Context, st *store.Store, pub outbox.Publisher, log *slog.Logger) {
	for {
		n, err := outbox.Relay(ctx, st.Pool(), 100, pub)
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

func openPublisher(log *slog.Logger) (outbox.Publisher, func(), error) {
	switch getenv("CONSENT_PUBLISHER", "kafka") {
	case "kafka":
		brokers := splitList(os.Getenv("KAFKA_BROKERS"))
		if len(brokers) == 0 {
			return nil, nil, errors.New("KAFKA_BROKERS is required when CONSENT_PUBLISHER=kafka")
		}
		kp, err := outbox.NewKafkaPublisher(brokers)
		if err != nil {
			return nil, nil, err
		}
		return kp, kp.Close, nil
	case "log":
		log.Warn("CONSENT_PUBLISHER=log: events are logged, not sent to Kafka; development only")
		return logPublisher{log}, func() {}, nil
	default:
		return nil, nil, errors.New("CONSENT_PUBLISHER must be kafka or log")
	}
}

type logPublisher struct{ log *slog.Logger }

func (p logPublisher) Publish(_ context.Context, topic string, key, value []byte) error {
	p.log.Info("event", "topic", topic, "key", string(key), "bytes", len(value))
	return nil
}

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
