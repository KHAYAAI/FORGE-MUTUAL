// Command core-bridge mirrors committed ledger transfers into Apache Fineract and reconciles the general ledger with the
// ledger every interval. It does not serve customer traffic.
package main

import (
	"connectrpc.com/connect"
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	ledgerv1 "github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1"
	"github.com/khayaai/forge-mutual/pkg/api/forge/ledger/v1/ledgerv1connect"
	"github.com/khayaai/forge-mutual/pkg/eventbus"
	"github.com/khayaai/forge-mutual/pkg/serve"
	"github.com/khayaai/forge-mutual/services/bank/core-bridge/internal/chart"
	"github.com/khayaai/forge-mutual/services/bank/core-bridge/internal/fineract"
	"github.com/khayaai/forge-mutual/services/bank/core-bridge/internal/poster"
	"github.com/khayaai/forge-mutual/services/bank/core-bridge/internal/projection"
	"github.com/khayaai/forge-mutual/services/bank/core-bridge/internal/recon"
	"github.com/khayaai/forge-mutual/services/bank/core-bridge/internal/store"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("core-bridge stopped", "err", err)
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

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("DATABASE_URL is required")
	}
	brokers := splitList(os.Getenv("KAFKA_BROKERS"))
	if len(brokers) == 0 {
		return errors.New("KAFKA_BROKERS is required")
	}
	ledgerURL := os.Getenv("LEDGER_URL")
	if ledgerURL == "" {
		return errors.New("LEDGER_URL is required")
	}
	fineractURL := os.Getenv("FINERACT_URL")
	if fineractURL == "" {
		return errors.New("FINERACT_URL is required, for example http://fineract:8080/fineract-provider/api/v1")
	}
	user, pass := os.Getenv("FINERACT_USER"), os.Getenv("FINERACT_PASSWORD")
	if user == "" || pass == "" {
		return errors.New("FINERACT_USER and FINERACT_PASSWORD are required, from the secret store")
	}
	postEvery, err := durationFromEnv("CORE_BRIDGE_POST_INTERVAL", 5*time.Second)
	if err != nil {
		return err
	}
	reconEvery, err := durationFromEnv("CORE_BRIDGE_RECON_INTERVAL", 24*time.Hour)
	if err != nil {
		return err
	}
	officeID, err := int64FromEnv("CORE_BRIDGE_OFFICE_ID", 1)
	if err != nil {
		return err
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := store.Open(ctx, pool); err != nil {
		return err
	}
	if err := eventbus.Migrate(ctx, pool); err != nil {
		return err
	}

	fin := &fineract.Client{
		BaseURL: fineractURL, Tenant: getenv("FINERACT_TENANT", "default"), User: user, Password: pass,
		HTTP: &http.Client{Timeout: 20 * time.Second},
	}
	gl, err := ensureGLAccounts(ctx, fin)
	if err != nil {
		return fmt.Errorf("general ledger accounts: %w", err)
	}

	ledger := ledgerv1connect.NewLedgerServiceClient(outbound, ledgerURL, callerAs("svc-core-bridge"))
	entity := getenv("CORE_BRIDGE_ENTITY", "BANK")
	refresh := func(ctx context.Context) error { return refreshAccounts(ctx, pool, ledger, entity) }
	resolver := projection.LedgerResolver{
		Lookup: func(ctx context.Context, id string) (store.AccountInfo, error) {
			return store.CachedAccount(ctx, pool, id)
		},
		Refresh: refresh,
	}

	consumer := &eventbus.Consumer{
		Name: "core-bridge", Pool: pool, Brokers: brokers, Group: "core-bridge",
		Topics: []string{projection.TopicTransferCommitted}, Handle: projection.Handler(resolver), Log: log,
	}
	go func() {
		if err := consumer.Run(ctx); err != nil {
			log.Error("ledger consumer stopped", "err", err)
			stop()
		}
	}()

	p := &poster.Poster{Pool: pool, Fin: fin, OfficeID: officeID, GL: gl, Log: log}
	go loop(ctx, log, "post", postEvery, func() error {
		posted, failed, err := p.Pass(ctx)
		if err == nil && posted+failed > 0 {
			log.Info("posting pass", "posted", posted, "failed", failed)
		}
		return err
	})

	rc := &recon.Recon{Pool: pool, Fin: fin, Ledger: ledger, Entity: entity, GL: gl}
	go loop(ctx, log, "reconcile", reconEvery, func() error {
		res, err := rc.Run(ctx)
		if err != nil {
			return err
		}
		log.Info("general ledger reconciliation", "checked", res.Checked, "breaks", res.Breaks, "skipped", res.Skipped)
		return nil
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			http.Error(w, "store unreachable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	stopTracing, err := serve.InitTracing(ctx, "core-bridge")
	if err != nil {
		return err
	}
	defer func() { _ = stopTracing(context.Background()) }()
	tlsCfg, err := serve.FromEnv(getenv("CORE_BRIDGE_ADDR", ":8098"))
	if err != nil {
		return err
	}
	srv, err := serve.New(tlsCfg, mux, log)
	if err != nil {
		return err
	}
	log.Info("core-bridge running", "office", officeID)
	return serve.Run(ctx, srv, log)
}

// ensureGLAccounts creates any missing mirrored GL account and returns the Fineract id of each.
func ensureGLAccounts(ctx context.Context, fin *fineract.Client) (map[string]int64, error) {
	existing, err := fin.GLAccounts(ctx)
	if err != nil {
		return nil, err
	}
	byCode := map[string]int64{}
	for _, a := range existing {
		byCode[a.Code] = a.ID
	}
	out := map[string]int64{}
	for _, asset := range chart.Assets {
		for _, e := range chart.Codes {
			code := chart.GLCode(e.Code, asset)
			id, ok := byCode[code]
			if !ok {
				id, err = fin.CreateGLAccount(ctx, code, e.Name+" ("+asset+")", int(e.Kind))
				if err != nil {
					return nil, fmt.Errorf("create %s: %w", code, err)
				}
			}
			out[code] = id
		}
	}
	return out, nil
}

// refreshAccounts copies the ledger's directory into the cache the projection reads.
func refreshAccounts(ctx context.Context, pool *pgxpool.Pool, ledger ledgerv1connect.LedgerServiceClient, entity string) error {
	cursor := ""
	for i := 0; i < 100000; i++ {
		resp, err := ledger.ListAccounts(ctx, connect.NewRequest(&ledgerv1.ListAccountsRequest{
			EntityCode: entity, Ledger: 0, AfterAccountId: cursor, Limit: 1000,
		}))
		if err != nil {
			return err
		}
		for _, a := range resp.Msg.Accounts {
			if err := store.CacheAccount(ctx, pool, a.AccountId, uint16(a.Code), a.Asset); err != nil {
				return err
			}
		}
		if resp.Msg.NextAfterAccountId == "" {
			return nil
		}
		cursor = resp.Msg.NextAfterAccountId
	}
	return errors.New("directory listing did not terminate")
}

// loop runs a job now and then on every tick. A failure is logged; the next tick tries again.
func loop(ctx context.Context, log *slog.Logger, name string, every time.Duration, job func() error) {
	tick := time.NewTicker(every)
	defer tick.Stop()
	for {
		if err := job(); err != nil {
			log.Error(name+" failed", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}

func callerAs(name string) connect.Option {
	return connect.WithInterceptors(connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			req.Header().Set("X-Forge-Caller", name)
			return next(ctx, req)
		}
	}))
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func durationFromEnv(name string, def time.Duration) (time.Duration, error) {
	v := os.Getenv(name)
	if v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", name)
	}
	return d, nil
}

func int64FromEnv(name string, def int64) (int64, error) {
	v := os.Getenv(name)
	if v == "" {
		return def, nil
	}
	var n int64
	if _, err := fmt.Sscan(v, &n); err != nil || n <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return n, nil
}

func getenv(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}
