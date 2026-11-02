// Command workspace-api is the backend for the customer workspace. It reads and writes through the services that own the
// data, and keeps no state of its own.
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

	"github.com/khayaai/forge-mutual/pkg/api/forge/consolidated/v1/consolidatedv1connect"
	"github.com/khayaai/forge-mutual/pkg/api/forge/fx/v1/fxv1connect"
	"github.com/khayaai/forge-mutual/pkg/api/forge/payments/v1/paymentsv1connect"
	"github.com/khayaai/forge-mutual/pkg/api/forge/payouts/v1/payoutsv1connect"
	"github.com/khayaai/forge-mutual/pkg/oidc"
	"github.com/khayaai/forge-mutual/services/platform/workspace-api/internal/api"
	"github.com/khayaai/forge-mutual/services/platform/workspace-api/internal/auth"
)

// callerName is what the downstream services see on every request this process makes. Each one lists it in its allowlist.
const callerName = "svc-workspace"

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("workspace-api stopped", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	authn, err := auth.Build(ctx, auth.Config{
		Mode: os.Getenv("WORKSPACE_AUTH"),
		Env:  getenv("FORGE_ENV", "development"),
		OIDC: oidcConfig(),
	})
	if err != nil {
		return err
	}
	urls := map[string]string{}
	for _, name := range []string{"CONSOLIDATED_URL", "PAYMENTS_URL", "FX_URL", "PAYOUTS_URL"} {
		v := os.Getenv(name)
		if v == "" {
			return fmt.Errorf("%s is required", name)
		}
		urls[name] = v
	}

	h := &api.API{
		Auth: authn,
		Services: api.Services{
			Consolidated: consolidatedv1connect.NewConsolidatedViewServiceClient(http.DefaultClient, urls["CONSOLIDATED_URL"], caller()),
			Payments:     paymentsv1connect.NewPaymentsServiceClient(http.DefaultClient, urls["PAYMENTS_URL"], caller()),
			FX:           fxv1connect.NewFXServiceClient(http.DefaultClient, urls["FX_URL"], caller()),
			Payouts:      payoutsv1connect.NewPayoutsServiceClient(http.DefaultClient, urls["PAYOUTS_URL"], caller()),
		},
	}
	srv := &http.Server{
		Addr:              getenv("WORKSPACE_ADDR", ":8300"),
		Handler:           h.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	log.Info("workspace-api listening", "addr", srv.Addr, "auth", os.Getenv("WORKSPACE_AUTH"))
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

// oidcConfig reads the identity provider's settings. OIDC_ALLOW_HTTP exists for local stacks only; production refuses it.
func oidcConfig() oidc.Config {
	return oidc.Config{
		Issuer:    os.Getenv("OIDC_ISSUER"),
		Audience:  os.Getenv("OIDC_AUDIENCE"),
		JWKSURL:   os.Getenv("OIDC_JWKS_URL"),
		AllowHTTP: os.Getenv("OIDC_ALLOW_HTTP") == "true",
	}
}
