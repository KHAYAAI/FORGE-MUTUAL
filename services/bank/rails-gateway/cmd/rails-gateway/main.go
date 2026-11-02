// Command rails-gateway sends payment instructions to a rail and reports their status
// (services/bank/rails-gateway/SPEC.md). In this build the rail is the simulator.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/khayaai/forge-mutual/pkg/serve"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"connectrpc.com/connect"

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
		connect.WithInterceptors(allow.Interceptor(callerauth.HeaderSource())))
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	listen := getenv("RAILS_LISTEN", ":8080")
	if err := serve.RefuseInProduction(serve.Switch{Name: "RAILS_SIMULATION", Set: os.Getenv("RAILS_SIMULATION") == "true"}); err != nil {
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
	log.Info("rails gateway listening", "addr", listen, "simulation", simulation)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	stopTracing, err := serve.InitTracing(ctx, "rails-gateway")
	if err != nil {
		return err
	}
	defer func() { _ = stopTracing(context.Background()) }()
	return serve.Run(ctx, srv, log)
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
