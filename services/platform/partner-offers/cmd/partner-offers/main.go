// Command partner-offers introduces customers to investment and crypto-asset products that a licensed third party provides.
// Every product is provided, licensed and monitored by that third party, never by Forge Mutual (see internal/offers).
//
// It is OFF unless PARTNER_OFFERS_ENABLED=true, and off in production unless FORGE_APPROVAL_PARTNER_OFFERS names the approving
// decision. No provider is chosen. Nothing in the customer app calls it.
//
// Configuration:
//
//	DATABASE_URL               Postgres (required)
//	CONSENT_URL                consent-service (required)
//	PARTY_URL                  party-service (required in production)
//	PARTNER_OFFERS_CALLERS_FILE  caller allow-list (required)
//	PARTNER_OFFERS_ADDR        listen address, default :8150
//	PARTNER_OFFERS_ENABLED     "true" to turn the layer on; anything else leaves it off
//	FORGE_APPROVAL_PARTNER_OFFERS  the production approval (pkg/licence)
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
	"syscall"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	consentv1 "github.com/khayaai/forge-mutual/pkg/api/forge/consent/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/consent/v1/consentv1connect"
	"github.com/khayaai/forge-mutual/pkg/api/forge/offers/v1/offersv1connect"
	partyv1 "github.com/khayaai/forge-mutual/pkg/api/forge/party/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/party/v1/partyv1connect"
	"github.com/khayaai/forge-mutual/pkg/callerauth"
	"github.com/khayaai/forge-mutual/pkg/licence"
	"github.com/khayaai/forge-mutual/pkg/serve"
	"github.com/khayaai/forge-mutual/services/platform/partner-offers/internal/offers"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("partner-offers stopped", "err", err)
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

	dsn, consentURL := os.Getenv("DATABASE_URL"), os.Getenv("CONSENT_URL")
	if dsn == "" || consentURL == "" {
		return errors.New("DATABASE_URL and CONSENT_URL are required")
	}
	callers, err := loadCallers(os.Getenv("PARTNER_OFFERS_CALLERS_FILE"))
	if err != nil {
		return err
	}
	gate, err := licence.FromEnv(os.Getenv)
	if err != nil {
		return err
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := offers.Open(ctx, pool); err != nil {
		return err
	}
	var parties offers.Parties
	if u := os.Getenv("PARTY_URL"); u != "" {
		parties = partyStatus{partyv1connect.NewPartyServiceClient(outbound, u, callerAs("svc-partner-offers"))}
	} else if callerauth.IsProduction() {
		return errors.New("PARTY_URL is required in production")
	}
	enabled := os.Getenv("PARTNER_OFFERS_ENABLED") == "true"
	svc := &offers.Service{Pool: pool, Parties: parties, Licence: gate, Enabled: enabled, Log: log,
		Consents: consentCheck{consentv1connect.NewConsentServiceClient(outbound, consentURL, callerAs("svc-partner-offers"))}}
	if enabled {
		log.Warn("third-party offers are ON: every product is provided, licensed and monitored by its provider, not by Forge")
	} else {
		log.Info("third-party offers are OFF (set PARTNER_OFFERS_ENABLED=true to turn them on)")
	}

	mux := http.NewServeMux()
	ic := connect.WithInterceptors(callers.Interceptor(callerauth.HeaderSource()))
	p1, h1 := offersv1connect.NewPartnerOffersServiceHandler(svc, ic)
	p2, h2 := offersv1connect.NewPartnerAdminServiceHandler(svc, ic)
	mux.Handle(p1, h1)
	mux.Handle(p2, h2)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			http.Error(w, "store unreachable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	stopTracing, err := serve.InitTracing(ctx, "partner-offers")
	if err != nil {
		return err
	}
	defer func() { _ = stopTracing(context.Background()) }()
	tlsCfg, err := serve.FromEnv(getenv("PARTNER_OFFERS_ADDR", ":8150"))
	if err != nil {
		return err
	}
	srv, err := serve.New(tlsCfg, mux, log)
	if err != nil {
		return err
	}
	log.Info("partner-offers listening", "addr", srv.Addr, "licence", gate.Status())
	return serve.Run(ctx, srv, log)
}

type consentCheck struct {
	client consentv1connect.ConsentServiceClient
}

func (c consentCheck) Allowed(ctx context.Context, party, purpose, scope string) (string, bool, error) {
	res, err := c.client.Check(ctx, connect.NewRequest(&consentv1.CheckRequest{PartyId: party, Purpose: purpose, Scope: scope}))
	if err != nil {
		return "", false, fmt.Errorf("consent: %w", err)
	}
	return res.Msg.ConsentId, res.Msg.Allowed, nil
}

type partyStatus struct {
	client partyv1connect.PartyServiceClient
}

func (p partyStatus) Active(ctx context.Context, party string) (bool, error) {
	res, err := p.client.GetParty(ctx, connect.NewRequest(&partyv1.GetPartyRequest{PartyId: party}))
	if err != nil {
		return false, fmt.Errorf("party status: %w", err)
	}
	return res.Msg.Party.GetStatus() == partyv1.PartyStatus_PARTY_STATUS_ACTIVE, nil
}

func callerAs(name string) connect.Option {
	return connect.WithInterceptors(connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			req.Header().Set(callerauth.DefaultHeader, name)
			return next(ctx, req)
		}
	}))
}

func loadCallers(path string) (callerauth.Allowlist, error) {
	if path == "" {
		return nil, errors.New("PARTNER_OFFERS_CALLERS_FILE is required")
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
			out[caller][m] = true
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
