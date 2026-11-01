// Command notifications turns platform events into transactional messages and delivers them.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/khayaai/forge-mutual/pkg/eventbus"
	"github.com/khayaai/forge-mutual/services/platform/notifications/internal/crypt"
	"github.com/khayaai/forge-mutual/services/platform/notifications/internal/handler"
	"github.com/khayaai/forge-mutual/services/platform/notifications/internal/sender"
	"github.com/khayaai/forge-mutual/services/platform/notifications/internal/store"
	"github.com/khayaai/forge-mutual/services/platform/notifications/internal/templates"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("notifications stopped", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
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
	key, err := crypt.ParseKey(os.Getenv("NOTIFICATIONS_KEY"))
	if err != nil {
		return errors.New("NOTIFICATIONS_KEY must be a base64 32-byte key, from the secret store")
	}
	sinkPath := os.Getenv("NOTIFICATIONS_SINK_FILE")
	if sinkPath == "" {
		return errors.New("NOTIFICATIONS_SINK_FILE is required until a real provider is configured (VERIFY)")
	}
	log.Warn("messages are written to a development file sink; real SMTP and SMS providers are not configured", "path", sinkPath)
	every, err := durationFromEnv("NOTIFICATIONS_SEND_INTERVAL", 5*time.Second)
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

	consumer := &eventbus.Consumer{
		Name: "notifications", Pool: pool, Brokers: brokers, Group: "notifications",
		Topics: templates.Topics, Handle: handler.Handle, Log: log,
	}
	go func() {
		if err := consumer.Run(ctx); err != nil {
			log.Error("event consumer stopped", "err", err)
			stop()
		}
	}()

	s := &sender.Sender{Pool: pool, Key: key, Provider: &sender.FileSink{Path: sinkPath}}
	go func() {
		tick := time.NewTicker(every)
		defer tick.Stop()
		for {
			if sent, failed, err := s.Pass(ctx); err != nil {
				log.Error("delivery pass failed", "err", err)
			} else if sent+failed > 0 {
				log.Info("delivery pass", "sent", sent, "failed", failed)
			}
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
			}
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			http.Error(w, "store unreachable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	srv := &http.Server{Addr: getenv("NOTIFICATIONS_ADDR", ":8097"), Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	select {
	case <-ctx.Done():
	case err := <-errc:
		return err
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
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

func getenv(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}
