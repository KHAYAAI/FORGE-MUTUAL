// Package outbox publishes committed events from the gateway's outbox table (ARCHITECTURE.md 7.8).
// Delivery is at least once. Consumers dedupe by event_id.
package outbox

import (
	"context"
	"log/slog"
	"time"

	"github.com/khayaai/forge-mutual/services/bank/ledger-gateway-bank/internal/gateway"
)

// Publisher sends one event to a topic and returns only once the broker has accepted it.
type Publisher interface {
	Publish(ctx context.Context, topic string, key, value []byte) error
}

// Relay drains the outbox.
type Relay struct {
	Repo      gateway.Repository
	Publisher Publisher
	BatchSize int
	Interval  time.Duration
	Log       *slog.Logger
}

// Once publishes one batch and returns how many events were published.
func (r *Relay) Once(ctx context.Context) (int, error) {
	return r.Repo.RelayBatch(ctx, r.BatchSize, func(events []gateway.OutboxEvent) error {
		for _, e := range events {
			if err := r.Publisher.Publish(ctx, e.Topic, e.Key, e.Payload); err != nil {
				return err
			}
		}
		return nil
	})
}

// Run publishes until ctx is cancelled. It drains backlog without waiting, and sleeps for
// Interval when the outbox is empty or a batch fails.
func (r *Relay) Run(ctx context.Context) {
	for {
		n, err := r.Once(ctx)
		switch {
		case ctx.Err() != nil:
			return
		case err != nil:
			r.Log.Error("outbox relay batch failed", "err", err)
			sleep(ctx, r.Interval)
		case n == 0:
			sleep(ctx, r.Interval)
		}
	}
}

func sleep(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}
