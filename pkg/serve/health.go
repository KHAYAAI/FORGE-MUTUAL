package serve

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// startHealth serves liveness and readiness probes on their own plain-HTTP listener, so an orchestrator can probe a
// service that requires client certificates on its main port (README: "a health listener that needs no client
// certificate"). Set HEALTH_ADDR (for example ":8181") to turn it on; with it unset nothing is opened.
//
// The listener answers two paths and nothing else, with no body beyond "ok", so it exposes no data and no
// functionality. In production, bind it to the pod's own address and let only the orchestrator's probes reach it
// (a network policy in deploy/k8s does this). An address that is not a host:port is refused.
//
//	/livez   200 while the process runs.
//	/readyz  200 until shutdown begins, then 503, so traffic drains before the main listener closes.
func startHealth(addr string, ready interface{ Load() bool }, log *slog.Logger) (func(), error) {
	if addr == "" {
		return func() {}, nil
	}
	if _, _, err := net.SplitHostPort(addr); err != nil {
		return nil, errors.New("HEALTH_ADDR must be host:port, for example :8181: " + err.Error())
	}
	mux := http.NewServeMux()
	ok := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte("ok"))
	}
	mux.HandleFunc("GET /livez", ok)
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if !ready.Load() {
			w.Header().Set("Cache-Control", "no-store")
			http.Error(w, "shutting down", http.StatusServiceUnavailable)
			return
		}
		ok(w, r)
	})
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 2 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, MaxHeaderBytes: 1 << 12}
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("health listener stopped", "err", err)
		}
	}()
	log.Info("health listener", "addr", ln.Addr().String())
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}, nil
}
