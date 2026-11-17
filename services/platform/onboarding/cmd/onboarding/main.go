// Command onboarding runs the business onboarding workflow worker and the onboarding API, including the
// KYC webhook receiver (services/platform/onboarding/SPEC.md).
package main

import (
	"bufio"
	"connectrpc.com/connect"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/khayaai/forge-mutual/pkg/api/forge/consent/v1/consentv1connect"
	"github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1/ledgerv1connect"
	"github.com/khayaai/forge-mutual/pkg/api/forge/onboarding/v1/onboardingv1connect"
	"github.com/khayaai/forge-mutual/pkg/api/forge/partner/v1/partnerv1connect"
	"github.com/khayaai/forge-mutual/pkg/api/forge/party/v1/partyv1connect"
	"github.com/khayaai/forge-mutual/pkg/api/forge/payments/v1/paymentsv1connect"
	"github.com/khayaai/forge-mutual/pkg/api/forge/policy/v1/policyv1connect"
	"github.com/khayaai/forge-mutual/pkg/callerauth"
	"github.com/khayaai/forge-mutual/pkg/licence"
	"github.com/khayaai/forge-mutual/pkg/screening"
	"github.com/khayaai/forge-mutual/pkg/serve"
	"github.com/khayaai/forge-mutual/services/platform/onboarding/internal/accounts"
	"github.com/khayaai/forge-mutual/services/platform/onboarding/internal/kyc"
	"github.com/khayaai/forge-mutual/services/platform/onboarding/internal/onboarding"
	"github.com/khayaai/forge-mutual/services/platform/onboarding/internal/service"
	"github.com/khayaai/forge-mutual/services/platform/onboarding/internal/store"
	"go.temporal.io/sdk/client"
	temporalotel "go.temporal.io/sdk/contrib/opentelemetry"
	"go.temporal.io/sdk/interceptor"
	"go.temporal.io/sdk/worker"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// callerAs stamps the outgoing requests with this service's identity.
func callerAs(name string) connect.Option {
	return connect.WithInterceptors(connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			req.Header().Set(callerauth.DefaultHeader, name)
			return next(ctx, req)
		}
	}))
}

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("onboarding stopped", "err", err)
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

	provider, err := providerFromEnv()
	if err != nil {
		return err
	}
	scr, err := screenerFromEnv()
	if err != nil {
		return err
	}
	callers, err := loadCallers(os.Getenv("ONBOARDING_CALLERS_FILE"))
	if err != nil {
		return err
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("DATABASE_URL is required: KYC reviews are kept in Postgres")
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()
	reviews, err := store.Open(ctx, pool)
	if err != nil {
		return err
	}

	hostPort := getenv("TEMPORAL_HOST", "localhost:7233")
	// The trace a request carries goes into the workflow it starts, and on through every activity's outbound calls,
	// so one trace shows a request from its caller to the ledger.
	tracing, err := temporalotel.NewTracingInterceptor(temporalotel.TracerOptions{})
	if err != nil {
		return fmt.Errorf("temporal tracing: %w", err)
	}
	tc, err := client.Dial(client.Options{Interceptors: []interceptor.ClientInterceptor{tracing}, HostPort: hostPort, Namespace: getenv("TEMPORAL_NAMESPACE", "default")})
	if err != nil {
		return fmt.Errorf("temporal: %w", err)
	}
	defer tc.Close()
	taskQueue := getenv("TEMPORAL_TASK_QUEUE", "onboarding")

	urls := map[string]string{}
	for _, name := range []string{"PARTY_URL", "CONSENT_URL", "POLICY_URL", "LEDGER_URL"} {
		v := os.Getenv(name)
		if v == "" {
			return fmt.Errorf("%s is required", name)
		}
		urls[name] = v
	}

	acts := &onboarding.Activities{
		Party:           partyv1connect.NewPartyServiceClient(outbound, urls["PARTY_URL"], callerAs("svc-onboarding")),
		Consent:         consentv1connect.NewConsentServiceClient(outbound, urls["CONSENT_URL"], callerAs("svc-onboarding")),
		Policy:          policyv1connect.NewPolicyServiceClient(outbound, urls["POLICY_URL"], callerAs("svc-onboarding")),
		Ledger:          ledgerv1connect.NewLedgerServiceClient(outbound, urls["LEDGER_URL"], callerAs("svc-onboarding")),
		KYC:             provider,
		Screener:        scr,
		InsuredDeposits: os.Getenv("INSURED_DEPOSITS") == "true",
		PolicyEntity:    getenv("POLICY_ENTITY", "PLAT"),
		Reviews:         reviews,
		Sessions:        reviews,
	}

	w := worker.New(tc, taskQueue, worker.Options{})
	w.RegisterWorkflow(onboarding.OnboardBusiness)
	w.RegisterWorkflow(onboarding.OnboardPerson)
	w.RegisterActivity(acts)
	if err := w.Start(); err != nil {
		return fmt.Errorf("worker: %w", err)
	}
	defer w.Stop()

	svc := service.New(tc, provider, taskQueue).WithReviews(reviews).WithSessions(reviews).WithLogger(log)
	// Further currency accounts. They need payments, to register the payer's reference; a foreign currency also needs
	// the partner-bank router. Without PAYMENTS_URL, OpenCurrencyAccount is unavailable.
	if paymentsURL := os.Getenv("PAYMENTS_URL"); paymentsURL != "" {
		gate, err := licence.FromEnv(os.Getenv)
		if err != nil {
			return err
		}
		opener := &accounts.Opener{
			Pool: pool, Parties: acts.Party, Ledger: acts.Ledger, Licence: gate, Entity: "BANK",
			References: paymentsv1connect.NewInboundReferenceServiceClient(outbound, paymentsURL, callerAs("svc-onboarding")),
			Insured:    os.Getenv("INSURED_DEPOSITS") == "true",
		}
		if partnerURL := os.Getenv("PARTNER_URL"); partnerURL != "" {
			opener.Partners = partnerv1connect.NewPartnerRouterServiceClient(outbound, partnerURL, callerAs("svc-onboarding"))
		}
		svc = svc.WithAccounts(opener)
		log.Info("currency accounts enabled", "foreign", opener.Partners != nil, "licence", gate.Status())
	}
	// A decision is recorded before the workflow is told. This tells it again if the first attempt failed.
	go svc.RunRedelivery(ctx, 15*time.Second, log)
	path, handler := onboardingv1connect.NewOnboardingServiceHandler(svc,
		connect.WithInterceptors(callers.Interceptor(callerauth.HeaderSource())))
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	mux.Handle("/webhooks/kyc", svc.WebhookHandler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	listen := getenv("ONBOARDING_LISTEN", ":8080")
	stopTracing, err := serve.InitTracing(ctx, "onboarding")
	if err != nil {
		return err
	}
	defer func() { _ = stopTracing(context.Background()) }()
	if err := serve.RefuseInProduction(
		serve.Switch{Name: "KYC_PROVIDER=fake", Set: getenv("KYC_PROVIDER", "didit") == "fake"},
		// A Didit sandbox application mocks its providers and can approve anyone.
		serve.Switch{Name: "DIDIT_ENVIRONMENT=sandbox", Set: os.Getenv("DIDIT_ENVIRONMENT") == "sandbox"},
	); err != nil {
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
	log.Info("onboarding listening", "addr", listen, "task_queue", taskQueue, "kyc", os.Getenv("KYC_PROVIDER"))
	return serve.Run(ctx, srv, log)
}

// providerFromEnv selects the identity provider. The fake needs an explicit secret. Didit needs its API key, its two
// workflows, its webhook secret and the environment its application runs in; without them the service does not start,
// so an unconfigured provider can never approve anyone. Secrets are read from the environment and never logged.
//
//	DIDIT_API_KEY                 the application's API key
//	DIDIT_KYC_WORKFLOW_ID         the workflow for a person (a UUID from the Didit console)
//	DIDIT_KYB_WORKFLOW_ID         the workflow for a business
//	DIDIT_WEBHOOK_SECRET          the webhook destination's secret_shared_key
//	DIDIT_WEBHOOK_SECRET_PREVIOUS the old secret, accepted too while it is being rotated (optional)
//	DIDIT_ENVIRONMENT             "sandbox" or "live"; production accepts only "live"
//	DIDIT_BASE_URL                default https://verification.didit.me (optional)
//	DIDIT_CALLBACK_URL            where Didit sends the person when they finish (optional)
func providerFromEnv() (kyc.Provider, error) {
	switch getenv("KYC_PROVIDER", "didit") {
	case "fake":
		secret := os.Getenv("KYC_WEBHOOK_SECRET")
		if secret == "" {
			return nil, errors.New("KYC_WEBHOOK_SECRET is required for the fake provider")
		}
		return kyc.NewFake([]byte(secret))
	case "didit":
		d, err := kyc.NewDidit(kyc.DiditConfig{
			APIKey:                os.Getenv("DIDIT_API_KEY"),
			KYCWorkflowID:         os.Getenv("DIDIT_KYC_WORKFLOW_ID"),
			KYBWorkflowID:         os.Getenv("DIDIT_KYB_WORKFLOW_ID"),
			WebhookSecret:         os.Getenv("DIDIT_WEBHOOK_SECRET"),
			PreviousWebhookSecret: os.Getenv("DIDIT_WEBHOOK_SECRET_PREVIOUS"),
			Environment:           os.Getenv("DIDIT_ENVIRONMENT"),
			BaseURL:               os.Getenv("DIDIT_BASE_URL"),
			CallbackURL:           os.Getenv("DIDIT_CALLBACK_URL"),
		})
		if err != nil {
			return nil, fmt.Errorf("KYC_PROVIDER=didit: %w", err)
		}
		return d, nil
	default:
		return nil, errors.New("KYC_PROVIDER must be fake or didit")
	}
}

// screenerFromEnv loads the sanctions list. There is no default list: without one, onboarding cannot
// start, because screening must fail closed.
func screenerFromEnv() (screening.Screener, error) {
	path := os.Getenv("SCREENING_LIST_FILE")
	if path == "" {
		return nil, errors.New("SCREENING_LIST_FILE is required; screening never runs without a list")
	}
	asOf, err := time.Parse(time.RFC3339, os.Getenv("SCREENING_LIST_AS_OF"))
	if err != nil {
		return nil, fmt.Errorf("SCREENING_LIST_AS_OF must be RFC 3339: %w", err)
	}
	maxAge, err := time.ParseDuration(getenv("SCREENING_MAX_AGE", "24h"))
	if err != nil {
		return nil, fmt.Errorf("SCREENING_MAX_AGE: %w", err)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var names []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if line := strings.TrimSpace(sc.Text()); line != "" && !strings.HasPrefix(line, "#") {
			names = append(names, line)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return screening.NewFixture(names, asOf, maxAge, time.Now)
}

func loadCallers(path string) (callerauth.Allowlist, error) {
	if path == "" {
		return nil, errors.New("ONBOARDING_CALLERS_FILE is required")
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

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
