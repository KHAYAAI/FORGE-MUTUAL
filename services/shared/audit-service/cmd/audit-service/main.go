// Command audit-service serves the append-only audit log and writes head anchors on a schedule.
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

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"connectrpc.com/connect"

	"github.com/khayaai/forge-mutual/pkg/api/forge/audit/v1/auditv1connect"
	"github.com/khayaai/forge-mutual/pkg/callerauth"
	"github.com/khayaai/forge-mutual/services/shared/audit-service/internal/anchor"
	"github.com/khayaai/forge-mutual/services/shared/audit-service/internal/service"
	"github.com/khayaai/forge-mutual/services/shared/audit-service/internal/store"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("audit-service stopped", "err", err)
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
	callers, err := loadCallers(os.Getenv("AUDIT_CALLERS_FILE"))
	if err != nil {
		return err
	}
	every, err := durationFromEnv("AUDIT_ANCHOR_EVERY", 10*time.Minute)
	if err != nil {
		return err
	}
	anchorPath := os.Getenv("AUDIT_ANCHOR_FILE")
	if anchorPath == "" {
		return errors.New("AUDIT_ANCHOR_FILE is required; anchors are what detect a rewritten history")
	}
	log.Warn("anchors are written to a local file, which is for development only; production needs write-once storage (VERIFY-WORM)", "path", anchorPath)

	st, err := store.Open(ctx, dsn)
	if err != nil {
		return err
	}
	defer st.Close()

	go anchorLoop(ctx, log, &anchor.Anchorer{Store: st, Sink: anchor.FileSink{Path: anchorPath}}, every)

	mux := http.NewServeMux()
	path, h := auditv1connect.NewAuditServiceHandler(&service.Service{Store: st},
		connect.WithInterceptors(callers.Interceptor(callerauth.DefaultHeader)))
	mux.Handle(path, h)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := st.Pool().Ping(r.Context()); err != nil {
			http.Error(w, "store unreachable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	srv := &http.Server{
		Addr:              getenv("AUDIT_ADDR", ":8094"),
		Handler:           h2c.NewHandler(mux, &http2.Server{}),
		ReadHeaderTimeout: 5 * time.Second,
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	log.Info("audit-service listening", "addr", srv.Addr)
	select {
	case <-ctx.Done():
	case err := <-errc:
		return err
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

func anchorLoop(ctx context.Context, log *slog.Logger, a *anchor.Anchorer, every time.Duration) {
	tick := time.NewTicker(every)
	defer tick.Stop()
	for {
		if an, wrote, err := a.Once(ctx); err != nil {
			log.Error("anchor failed", "err", err)
		} else if wrote {
			log.Info("anchored", "seq", an.Seq)
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

func loadCallers(path string) (callerauth.Allowlist, error) {
	if path == "" {
		return nil, errors.New("AUDIT_CALLERS_FILE is required")
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

func getenv(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}
