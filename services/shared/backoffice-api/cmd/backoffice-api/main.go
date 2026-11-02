// Command backoffice-api is the back office's API for staff. It reads and writes through the services that own the data,
// and keeps no state of its own.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"connectrpc.com/connect"

	"github.com/khayaai/forge-mutual/pkg/api/forge/cases/v1/casesv1connect"
	"github.com/khayaai/forge-mutual/pkg/api/forge/payments/v1/paymentsv1connect"
	"github.com/khayaai/forge-mutual/pkg/api/forge/recon/v1/reconv1connect"
	"github.com/khayaai/forge-mutual/services/shared/backoffice-api/internal/api"
	"github.com/khayaai/forge-mutual/services/shared/backoffice-api/internal/auth"
)

// callerName is what the downstream services see on every request this process makes. Each one lists it in its allowlist.
const callerName = "svc-backoffice-api"

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("backoffice-api stopped", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	authn, err := auth.Select(os.Getenv("BACKOFFICE_AUTH"), getenv("FORGE_ENV", "development"))
	if err != nil {
		return err
	}
	urls := map[string]string{}
	for _, name := range []string{"CASES_URL", "PAYMENTS_URL", "RECON_URL"} {
		v := os.Getenv(name)
		if v == "" {
			return fmt.Errorf("%s is required", name)
		}
		urls[name] = v
	}

	h := &api.API{
		Auth: authn,
		Services: api.Services{
			Cases:    casesv1connect.NewCasesServiceClient(http.DefaultClient, urls["CASES_URL"], caller()),
			Payments: paymentsv1connect.NewPaymentsServiceClient(http.DefaultClient, urls["PAYMENTS_URL"], caller()),
			Recon:    reconv1connect.NewReconServiceClient(http.DefaultClient, urls["RECON_URL"], caller()),
		},
	}
	srv := &http.Server{
		Addr:              getenv("BACKOFFICE_ADDR", ":8500"),
		Handler:           h.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	log.Info("backoffice-api listening", "addr", srv.Addr, "auth", os.Getenv("BACKOFFICE_AUTH"))
	select {
	case <-ctx.Done():
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// caller sets the caller header on every downstream request.
func caller() connect.Option {
	return connect.WithInterceptors(connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			req.Header().Set("X-Forge-Caller", callerName)
			return next(ctx, req)
		}
	}))
}

func getenv(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}
