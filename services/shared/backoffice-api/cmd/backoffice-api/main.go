// Command backoffice-api is the back office's API for staff. It reads and writes through the services that own the data,
// and keeps no state of its own.
package main

import (
	"connectrpc.com/connect"
	"context"
	"fmt"
	"github.com/khayaai/forge-mutual/pkg/api/forge/cases/v1/casesv1connect"
	"github.com/khayaai/forge-mutual/pkg/api/forge/onboarding/v1/onboardingv1connect"
	"github.com/khayaai/forge-mutual/pkg/api/forge/payments/v1/paymentsv1connect"
	"github.com/khayaai/forge-mutual/pkg/api/forge/recon/v1/reconv1connect"
	"github.com/khayaai/forge-mutual/pkg/oidc"
	"github.com/khayaai/forge-mutual/pkg/serve"
	"github.com/khayaai/forge-mutual/services/shared/backoffice-api/internal/api"
	"github.com/khayaai/forge-mutual/services/shared/backoffice-api/internal/auth"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
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
	outbound, err := serve.OutboundFromEnv()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	authn, err := auth.Build(ctx, auth.Config{
		Mode: os.Getenv("BACKOFFICE_AUTH"),
		Env:  getenv("FORGE_ENV", "development"),
		OIDC: oidcConfig(),
	})
	if err != nil {
		return err
	}
	urls := map[string]string{}
	for _, name := range []string{"CASES_URL", "PAYMENTS_URL", "RECON_URL", "ONBOARDING_URL"} {
		v := os.Getenv(name)
		if v == "" {
			return fmt.Errorf("%s is required", name)
		}
		urls[name] = v
	}

	h := &api.API{
		Auth: authn,
		Services: api.Services{
			Cases:      casesv1connect.NewCasesServiceClient(outbound, urls["CASES_URL"], caller()),
			Payments:   paymentsv1connect.NewPaymentsServiceClient(outbound, urls["PAYMENTS_URL"], caller()),
			Recon:      reconv1connect.NewReconServiceClient(outbound, urls["RECON_URL"], caller()),
			Onboarding: onboardingv1connect.NewOnboardingServiceClient(outbound, urls["ONBOARDING_URL"], caller()),
		},
	}
	stopTracing, err := serve.InitTracing(ctx, "backoffice-api")
	if err != nil {
		return err
	}
	defer func() { _ = stopTracing(context.Background()) }()
	tlsCfg, err := serve.FromEnv(getenv("BACKOFFICE_ADDR", ":8500"))
	if err != nil {
		return err
	}
	srv, err := serve.New(tlsCfg, h.Handler(), log)
	if err != nil {
		return err
	}
	log.Info("backoffice-api listening", "addr", srv.Addr, "auth", os.Getenv("BACKOFFICE_AUTH"))
	return serve.Run(ctx, srv, log)
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
