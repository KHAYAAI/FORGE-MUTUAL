// Command workspace-api is the backend for the customer workspace. It reads and writes through the services that own the
// data, and keeps no state of its own.
package main

import (
	"connectrpc.com/connect"
	"context"
	"fmt"
	"github.com/khayaai/forge-mutual/pkg/api/forge/consolidated/v1/consolidatedv1connect"
	"github.com/khayaai/forge-mutual/pkg/api/forge/deposits/v1/depositsv1connect"
	"github.com/khayaai/forge-mutual/pkg/api/forge/fx/v1/fxv1connect"
	"github.com/khayaai/forge-mutual/pkg/api/forge/onboarding/v1/onboardingv1connect"
	"github.com/khayaai/forge-mutual/pkg/api/forge/payments/v1/paymentsv1connect"
	"github.com/khayaai/forge-mutual/pkg/api/forge/payouts/v1/payoutsv1connect"
	"github.com/khayaai/forge-mutual/pkg/oidc"
	"github.com/khayaai/forge-mutual/pkg/serve"
	"github.com/khayaai/forge-mutual/services/platform/workspace-api/internal/api"
	"github.com/khayaai/forge-mutual/services/platform/workspace-api/internal/auth"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
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
	outbound, err := serve.OutboundFromEnv()
	if err != nil {
		return err
	}
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
	for _, name := range []string{"CONSOLIDATED_URL", "PAYMENTS_URL", "FX_URL", "PAYOUTS_URL", "ONBOARDING_URL"} {
		v := os.Getenv(name)
		if v == "" {
			return fmt.Errorf("%s is required", name)
		}
		urls[name] = v
	}

	h := &api.API{
		Auth: authn,
		Services: api.Services{
			Consolidated: consolidatedv1connect.NewConsolidatedViewServiceClient(outbound, urls["CONSOLIDATED_URL"], caller()),
			Payments:     paymentsv1connect.NewPaymentsServiceClient(outbound, urls["PAYMENTS_URL"], caller()),
			FX:           fxv1connect.NewFXServiceClient(outbound, urls["FX_URL"], caller()),
			Payouts:      payoutsv1connect.NewPayoutsServiceClient(outbound, urls["PAYOUTS_URL"], caller()),
			Onboarding:   onboardingv1connect.NewOnboardingServiceClient(outbound, urls["ONBOARDING_URL"], caller()),
			CrossBorder:  paymentsv1connect.NewCrossBorderServiceClient(outbound, urls["PAYMENTS_URL"], caller()),
			References:   paymentsv1connect.NewInboundReferenceServiceClient(outbound, urls["PAYMENTS_URL"], caller()),
			Deposits:     depositsClient(outbound, caller()),
		},
	}
	stopTracing, err := serve.InitTracing(ctx, "workspace-api")
	if err != nil {
		return err
	}
	defer func() { _ = stopTracing(context.Background()) }()
	tlsCfg, err := serve.FromEnv(getenv("WORKSPACE_ADDR", ":8300"))
	if err != nil {
		return err
	}
	// This API is for one caller: the customer web app's server. With mutual TLS on, every service the CA has signed could
	// otherwise reach it and present a person's token. WORKSPACE_CLIENTS names the services whose certificates are served
	// (comma-separated), and production refuses to start without it.
	handler, err := serve.OnlyClients(tlsCfg, "WORKSPACE_CLIENTS", serve.ClientsFromEnv("WORKSPACE_CLIENTS"), h.Handler())
	if err != nil {
		return err
	}
	srv, err := serve.New(tlsCfg, handler, log)
	if err != nil {
		return err
	}
	log.Info("workspace-api listening", "addr", srv.Addr, "auth", os.Getenv("WORKSPACE_AUTH"))
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

// depositsClient is the deposits-service client, or nil when DEPOSITS_URL is not set (savings are not offered).
func depositsClient(h connect.HTTPClient, opt connect.ClientOption) depositsv1connect.DepositsServiceClient {
	u := os.Getenv("DEPOSITS_URL")
	if u == "" {
		return nil
	}
	return depositsv1connect.NewDepositsServiceClient(h, u, opt)
}
