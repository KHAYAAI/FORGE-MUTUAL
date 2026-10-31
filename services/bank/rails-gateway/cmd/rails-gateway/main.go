// Command rails-gateway sends payment instructions to a rail and reports their status
// (services/bank/rails-gateway/SPEC.md). In this build the rail is the simulator.
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

	"github.com/khayaai/forge-mutual/pkg/api/forge/rails/v1/railsv1connect"
	"github.com/khayaai/forge-mutual/pkg/callerauth"
	"github.com/khayaai/forge-mutual/services/bank/rails-gateway/internal/rail"
	"github.com/khayaai/forge-mutual/services/bank/rails-gateway/internal/service"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("rails gateway stopped", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	callersFile := os.Getenv("RAILS_CALLERS_FILE")
	if callersFile == "" {
		return errors.New("RAILS_CALLERS_FILE is required")
	}
	allow, err := loadCallers(callersFile)
	if err != nil {
		return err
	}
	// The only rail in this build is the simulator. Scenarios are honoured only when simulation is on,
	// and the flag must be set explicitly.
	simulation := os.Getenv("RAILS_SIMULATION") == "true"
	if simulation {
		log.Warn("RAILS_SIMULATION=true: the rail is a simulator and scenarios are honoured; not for production")
	}
	r := rail.NewSim(simulation)

	path, handler := railsv1connect.NewRailServiceHandler(service.New(r),
		connect.WithInterceptors(allow.Interceptor(callerauth.DefaultHeader)))
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	listen := getenv("RAILS_LISTEN", ":8080")
	srv := &http.Server{Addr: listen, Handler: h2c.NewHandler(mux, &http2.Server{}), ReadHeaderTimeout: 5 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	errc := make(chan error, 1)
	go func() {
		log.Info("rails gateway listening", "addr", listen, "simulation", simulation)
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
