// Package eventbus consumes events from Kafka with effectively-once processing. Each event is handled in a Postgres
// transaction that also records the event id for this consumer. A redelivered event is skipped once its transaction
// has committed, so a crash between the commit and the Kafka offset does not apply the event twice. A message that
// cannot be decoded, or that keeps failing, goes to a dead-letter table, so it cannot block its partition forever.
package eventbus

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/twmb/franz-go/pkg/kgo"

	eventsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/events/v1"
	"github.com/khayaai/forge-mutual/pkg/events"
)

//go:embed schema.sql
var schema string

// Handler applies one event inside the transaction that records it. It must do its writes through tx.
type Handler func(ctx context.Context, tx pgx.Tx, env *eventsv1.EventEnvelope) error

// Consumer is one consumer group in one service.
type Consumer struct {
	Name     string // identifies this consumer in the processed and dead-letter tables
	Pool     *pgxpool.Pool
	Brokers  []string
	Group    string
	Topics   []string
	Handle   Handler
	Attempts int           // handler attempts before dead-lettering; default 5
	Backoff  time.Duration // between attempts; default 500ms
	Log      *slog.Logger
	// Ready, if set, is closed once the group has joined and this consumer has been assigned partitions.
	Ready chan struct{}
}

// Migrate creates the processed-event and dead-letter tables.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, schema); err != nil {
		return fmt.Errorf("eventbus: migrate: %w", err)
	}
	return nil
}

// Run consumes until the context ends. It returns nil on cancellation.
func (c *Consumer) Run(ctx context.Context) error {
	if c.Name == "" || c.Group == "" || len(c.Topics) == 0 || c.Handle == nil || c.Pool == nil {
		return errors.New("eventbus: name, group, topics, handler and pool are required")
	}
	if c.Attempts <= 0 {
		c.Attempts = 5
	}
	if c.Backoff <= 0 {
		c.Backoff = 500 * time.Millisecond
	}
	log := c.Log
	if log == nil {
		log = slog.Default()
	}
	// Ready is signalled on partition assignment, not on the first poll: a poll on an idle topic blocks until a record
	// arrives, so a signal sent after it would never come on a quiet topic.
	var readyOnce sync.Once
	cl, err := kgo.NewClient(
		kgo.SeedBrokers(c.Brokers...),
		kgo.ConsumerGroup(c.Group),
		kgo.ConsumeTopics(c.Topics...),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
		kgo.DisableAutoCommit(),
		kgo.OnPartitionsAssigned(func(context.Context, *kgo.Client, map[string][]int32) {
			if c.Ready != nil {
				readyOnce.Do(func() { close(c.Ready) })
			}
		}),
	)
	if err != nil {
		return fmt.Errorf("eventbus: client: %w", err)
	}
	defer cl.Close()

	for {
		fetches := cl.PollFetches(ctx)
		if ctx.Err() != nil {
			return nil
		}
		if errs := fetches.Errors(); len(errs) > 0 {
			for _, e := range errs {
				log.Warn("fetch error", "topic", e.Topic, "err", e.Err)
			}
		}
		var handleErr error
		fetches.EachRecord(func(r *kgo.Record) {
			if handleErr != nil {
				return
			}
			if err := c.process(ctx, log, r); err != nil {
				handleErr = err
				return
			}
			if err := cl.CommitRecords(ctx, r); err != nil && ctx.Err() == nil {
				log.Warn("commit failed; the event will be redelivered and skipped", "err", err)
			}
		})
		if handleErr != nil {
			return handleErr
		}
	}
}

// process handles one record: decode, skip if already processed, otherwise handle and record in one transaction.
func (c *Consumer) process(ctx context.Context, log *slog.Logger, r *kgo.Record) error {
	env, err := events.Open(r.Value)
	if err != nil {
		return c.deadLetter(ctx, r, fmt.Errorf("undecodable: %w", err))
	}
	var lastErr error
	for attempt := 1; attempt <= c.Attempts; attempt++ {
		lastErr = pgx.BeginFunc(ctx, c.Pool, func(tx pgx.Tx) error {
			tag, err := tx.Exec(ctx, `
				insert into eventbus_processed (consumer, event_id) values ($1, $2)
				on conflict do nothing`, c.Name, env.EventId)
			if err != nil {
				return err
			}
			if tag.RowsAffected() == 0 {
				return nil // already processed: skip the handler
			}
			return c.Handle(ctx, tx, env)
		})
		if lastErr == nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		log.Warn("handler failed; will retry", "consumer", c.Name, "event", env.EventId, "attempt", attempt, "err", lastErr)
		time.Sleep(c.Backoff)
	}
	return c.deadLetter(ctx, r, lastErr)
}

// deadLetter records a record that cannot be applied, so the partition can move on. The record is kept, not dropped.
func (c *Consumer) deadLetter(ctx context.Context, r *kgo.Record, cause error) error {
	_, err := c.Pool.Exec(ctx, `
		insert into eventbus_dead_letter (consumer, topic, partition, kafka_offset, error, value)
		values ($1, $2, $3, $4, $5, $6)`,
		c.Name, r.Topic, r.Partition, r.Offset, cause.Error(), r.Value)
	if err != nil {
		return fmt.Errorf("eventbus: dead letter: %w", err)
	}
	return nil
}
