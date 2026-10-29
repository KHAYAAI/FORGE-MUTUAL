// Command onboarding runs the business onboarding workflow worker and the onboarding API, including the
// KYC webhook receiver (services/platform/onboarding/SPEC.md).
package main

import (
	"bufio"
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
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/khayaai/forge-mutual/pkg/api/forge/consent/v1/consentv1connect"
	"github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1/ledgerv1connect"
	"github.com/khayaai/forge-mutual/pkg/api/forge/onboarding/v1/onboardingv1connect"
	"github.com/khayaai/forge-mutual/pkg/api/forge/party/v1/partyv1connect"
	"github.com/khayaai/forge-mutual/pkg/api/forge/policy/v1/policyv1connect"
	"github.com/khayaai/forge-mutual/pkg/callerauth"
	"github.com/khayaai/forge-mutual/services/platform/onboarding/internal/kyc"
	"github.com/khayaai/forge-mutual/services/platform/onboarding/internal/onboarding"
	"github.com/khayaai/forge-mutual/services/platform/onboarding/internal/screening"
	"github.com/khayaai/forge-mutual/services/platform/onboarding/internal/service"
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

	hostPort := getenv("TEMPORAL_HOST", "localhost:7233")
	tc, err := client.Dial(client.Options{HostPort: hostPort, Namespace: getenv("TEMPORAL_NAMESPACE", "default")})
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
		Party:           partyv1connect.NewPartyServiceClient(http.DefaultClient, urls["PARTY_URL"], callerAs("svc-onboarding")),
		Consent:         consentv1connect.NewConsentServiceClient(http.DefaultClient, urls["CONSENT_URL"], callerAs("svc-onboarding")),
		Policy:          policyv1connect.NewPolicyServiceClient(http.DefaultClient, urls["POLICY_URL"], callerAs("svc-onboarding")),
		Ledger:          ledgerv1connect.NewLedgerServiceClient(http.DefaultClient, urls["LEDGER_URL"], callerAs("svc-onboarding")),
		KYC:             provider,
		Screener:        scr,
		InsuredDeposits: os.Getenv("INSURED_DEPOSITS") == "true",
		PolicyEntity:    getenv("POLICY_ENTITY", "PLAT"),
	}

	w := worker.New(tc, taskQueue, worker.Options{})
	w.RegisterWorkflow(onboarding.OnboardBusiness)
	w.RegisterActivity(acts)
	if err := w.Start(); err != nil {
		return fmt.Errorf("worker: %w", err)
	}
	defer w.Stop()

	svc := service.New(tc, provider, taskQueue)
	path, handler := onboardingv1connect.NewOnboardingServiceHandler(svc,
		connect.WithInterceptors(callers.Interceptor(callerauth.DefaultHeader)))
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	mux.Handle("/webhooks/kyc", svc.WebhookHandler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	listen := getenv("ONBOARDING_LISTEN", ":8080")
	srv := &http.Server{Addr: listen, Handler: h2c.NewHandler(mux, &http2.Server{}), ReadHeaderTimeout: 5 * time.Second}
	errc := make(chan error, 1)
	go func() {
		log.Info("onboarding listening", "addr", listen, "task_queue", taskQueue, "kyc", os.Getenv("KYC_PROVIDER"))
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

// providerFromEnv selects the identity provider. The fake needs an explicit secret; Didit is not
// configured in this build and refuses every verification.
func providerFromEnv() (kyc.Provider, error) {
	switch getenv("KYC_PROVIDER", "didit") {
	case "fake":
		secret := os.Getenv("KYC_WEBHOOK_SECRET")
		if secret == "" {
			return nil, errors.New("KYC_WEBHOOK_SECRET is required for the fake provider")
		}
		return kyc.NewFake([]byte(secret))
	case "didit":
		return kyc.Didit{}, nil
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
