// Command card-service issues virtual debit cards and decides purchases in real time: the cardholder API, the
// processor's signed webhook, and recovery of interrupted authorisations.
//
// Configuration:
//
//	DATABASE_URL              Postgres (required)
//	LEDGER_URL                ledger-gateway-bank (required)
//	CONSOLIDATED_URL          consolidated-view, for which accounts a party owns (required)
//	PARTY_URL                 party-service, so a frozen party cannot spend (required in production)
//	CARDS_CALLERS_FILE        caller allow-list (required)
//	CARDS_ADDR                listen address, default :8140
//	CARDS_RECOVER_INTERVAL    how often interrupted authorisations are finished, default 1m
//	CARDS_DEFAULT_PER_TXN_MINOR, CARDS_DEFAULT_DAILY_MINOR, CARDS_MAX_PER_TXN_MINOR, CARDS_MAX_DAILY_MINOR,
//	CARDS_MAX_PER_PARTY, CARDS_BLOCKED_MCC   limits; PROPOSED placeholders (docs/THRESHOLDS.md 29 and 30)
//	CARD_PROCESSOR            "sim", the only processor built (VERIFY-PROCESSOR); refused in production
//	CARD_WEBHOOK_SECRET       the simulator's webhook secret (or _FILE)
//	FORGE_APPROVAL_CARDS      the production approval (pkg/licence)
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
	"strconv"
	"strings"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/khayaai/forge-mutual/pkg/api/forge/cards/v1/cardsv1connect"
	consolidatedv1 "github.com/khayaai/forge-mutual/pkg/api/forge/consolidated/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/consolidated/v1/consolidatedv1connect"
	"github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1/ledgerv1connect"
	partyv1 "github.com/khayaai/forge-mutual/pkg/api/forge/party/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/party/v1/partyv1connect"
	"github.com/khayaai/forge-mutual/pkg/callerauth"
	"github.com/khayaai/forge-mutual/pkg/licence"
	"github.com/khayaai/forge-mutual/pkg/serve"
	"github.com/khayaai/forge-mutual/services/bank/card-service/internal/cards"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("card-service stopped", "err", err)
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

	dsn, ledgerURL, consolidatedURL := os.Getenv("DATABASE_URL"), os.Getenv("LEDGER_URL"), os.Getenv("CONSOLIDATED_URL")
	if dsn == "" || ledgerURL == "" || consolidatedURL == "" {
		return errors.New("DATABASE_URL, LEDGER_URL and CONSOLIDATED_URL are required")
	}
	callers, err := loadCallers(os.Getenv("CARDS_CALLERS_FILE"))
	if err != nil {
		return err
	}
	gate, err := licence.FromEnv(os.Getenv)
	if err != nil {
		return err
	}
	every, err := time.ParseDuration(getenv("CARDS_RECOVER_INTERVAL", "1m"))
	if err != nil || every <= 0 {
		return errors.New("CARDS_RECOVER_INTERVAL must be a positive duration")
	}
	if os.Getenv("CARD_PROCESSOR") != "sim" {
		return errors.New("CARD_PROCESSOR must be \"sim\": no card processor or programme manager is chosen yet (VERIFY-PROCESSOR)")
	}
	if err := serve.RefuseInProduction(serve.Switch{Name: "CARD_PROCESSOR=sim", Set: true}); err != nil {
		return err
	}
	secret := os.Getenv("CARD_WEBHOOK_SECRET")
	if len(secret) < 16 {
		return errors.New("CARD_WEBHOOK_SECRET must be at least 16 characters")
	}
	num := func(name, def string, lo, hi int64) (int64, error) {
		v, err := strconv.ParseInt(getenv(name, def), 10, 64)
		if err != nil || v < lo || v > hi {
			return 0, fmt.Errorf("%s must be %d to %d", name, lo, hi)
		}
		return v, nil
	}
	perTxn, err := num("CARDS_DEFAULT_PER_TXN_MINOR", "1000000", 1, 100_000_000) // R10 000
	if err != nil {
		return err
	}
	daily, err := num("CARDS_DEFAULT_DAILY_MINOR", "2500000", perTxn, 1_000_000_000) // R25 000
	if err != nil {
		return err
	}
	maxPerTxn, err := num("CARDS_MAX_PER_TXN_MINOR", "5000000", perTxn, 1_000_000_000) // R50 000
	if err != nil {
		return err
	}
	maxDaily, err := num("CARDS_MAX_DAILY_MINOR", "10000000", daily, 10_000_000_000) // R100 000
	if err != nil {
		return err
	}
	perParty, err := num("CARDS_MAX_PER_PARTY", "5", 1, 50)
	if err != nil {
		return err
	}
	blocked := map[string]bool{}
	for _, m := range strings.Split(getenv("CARDS_BLOCKED_MCC", "7995,7801,7802"), ",") { // gambling (PROPOSED)
		if m = strings.TrimSpace(m); m != "" {
			blocked[m] = true
		}
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := cards.Open(ctx, pool); err != nil {
		return err
	}
	var parties cards.Parties
	if u := os.Getenv("PARTY_URL"); u != "" {
		parties = partyStatus{partyv1connect.NewPartyServiceClient(outbound, u, callerAs("svc-cards"))}
	} else if callerauth.IsProduction() {
		return errors.New("PARTY_URL is required in production: a frozen party must not be able to spend")
	}
	svc := &cards.Service{
		Pool: pool, Ledger: ledgerv1connect.NewLedgerServiceClient(outbound, ledgerURL, callerAs("svc-cards")),
		Owners:  owners{consolidatedv1connect.NewConsolidatedViewServiceClient(outbound, consolidatedURL, callerAs("svc-cards"))},
		Parties: parties, Processor: &cards.Simulator{Secret: []byte(secret)}, Licence: gate, Entity: "BANK", Log: log,
		DefaultPerTxn: perTxn, DefaultDaily: daily, MaxPerTxn: maxPerTxn, MaxDaily: maxDaily, MaxCardsPerParty: int(perParty), BlockedMCC: blocked,
	}
	if err := svc.EnsureSystemAccounts(ctx); err != nil {
		return fmt.Errorf("card system accounts: %w", err)
	}
	log.Warn("the card processor is the simulator and the limits are placeholders (VERIFY-PROCESSOR)", "default_per_txn_minor", perTxn, "default_daily_minor", daily)

	go func() {
		tick := time.NewTicker(every)
		defer tick.Stop()
		for {
			if n, err := svc.Recover(ctx); err != nil {
				log.Error("recovery failed; will retry", "err", err)
			} else if n > 0 {
				log.Info("authorisations recovered", "count", n)
			}
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
			}
		}
	}()

	mux := http.NewServeMux()
	path, handler := cardsv1connect.NewCardServiceHandler(svc, connect.WithInterceptors(callers.Interceptor(callerauth.HeaderSource())))
	mux.Handle(path, handler)
	// The processor's webhook authenticates itself by its signature, not by a caller certificate.
	mux.HandleFunc("POST /webhooks/processor", svc.Webhook)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			http.Error(w, "store unreachable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	stopTracing, err := serve.InitTracing(ctx, "card-service")
	if err != nil {
		return err
	}
	defer func() { _ = stopTracing(context.Background()) }()
	tlsCfg, err := serve.FromEnv(getenv("CARDS_ADDR", ":8140"))
	if err != nil {
		return err
	}
	srv, err := serve.New(tlsCfg, mux, log)
	if err != nil {
		return err
	}
	log.Info("card-service listening", "addr", srv.Addr, "licence", gate.Status())
	return serve.Run(ctx, srv, log)
}

type owners struct {
	client consolidatedv1connect.ConsolidatedViewServiceClient
}

func (o owners) Accounts(ctx context.Context, party string) (map[string]string, error) {
	res, err := o.client.GetBalances(ctx, connect.NewRequest(&consolidatedv1.GetBalancesRequest{PartyId: party}))
	if err != nil {
		return nil, fmt.Errorf("ownership: %w", err)
	}
	out := map[string]string{}
	for _, a := range res.Msg.Accounts {
		out[a.AccountId] = a.Asset
	}
	return out, nil
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
		return nil, errors.New("CARDS_CALLERS_FILE is required")
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
