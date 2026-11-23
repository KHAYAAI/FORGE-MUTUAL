package outbox

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/khayaai/forge-mutual/pkg/uuid"
)

// These tests need a real broker and a throwaway database: KAFKA_TEST_BROKERS (for example 127.0.0.1:9092) and
// OUTBOX_TEST_DSN. They create their own topic and truncate the outbox table. `make ci-db` sets both.
func kafkaEnv(t *testing.T) ([]string, *pgxpool.Pool) {
	t.Helper()
	brokers := os.Getenv("KAFKA_TEST_BROKERS")
	dsn := os.Getenv("OUTBOX_TEST_DSN")
	if brokers == "" || dsn == "" {
		t.Skip("KAFKA_TEST_BROKERS and OUTBOX_TEST_DSN are not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `truncate outbox`); err != nil {
		t.Fatal(err)
	}
	return strings.Split(brokers, ","), pool
}

func newTopic(t *testing.T, brokers []string) string {
	t.Helper()
	name := fmt.Sprintf("forge.test.outbox.%d", time.Now().UnixNano())
	cl, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cl.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, err := kadm.NewClient(cl).CreateTopic(ctx, 1, 1, nil, name)
	if err != nil {
		t.Fatalf("create topic: %v", err)
	}
	if resp.Err != nil {
		t.Fatalf("create topic: %v", resp.Err)
	}
	return name
}

func appendEvents(t *testing.T, pool *pgxpool.Pool, topic string, n int) {
	t.Helper()
	ctx := context.Background()
	for i := 0; i < n; i++ {
		id, err := uuid.NewV4()
		if err != nil {
			t.Fatal(err)
		}
		e := Event{EventID: id, Topic: topic, Key: []byte(fmt.Sprintf("key-%d", i)), Payload: []byte(fmt.Sprintf("payload-%d", i))}
		if err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error { return Append(ctx, tx, e) }); err != nil {
			t.Fatal(err)
		}
	}
}

func unpublished(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), `select count(*) from outbox where published_at is null`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// Every appended event reaches the topic exactly once, in append order, and is marked published.
func TestRelayDeliversToRealKafkaInOrder(t *testing.T) {
	brokers, pool := kafkaEnv(t)
	topic := newTopic(t, brokers)
	appendEvents(t, pool, topic, 3)

	pub, err := NewKafkaPublisher(brokers)
	if err != nil {
		t.Fatal(err)
	}
	defer pub.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	n, err := Relay(ctx, pool, 100, pub)
	if err != nil || n != 3 {
		t.Fatalf("relay published %d, err %v; want 3", n, err)
	}
	if got := unpublished(t, pool); got != 0 {
		t.Fatalf("%d events still unpublished after a successful relay", got)
	}

	consumer, err := kgo.NewClient(kgo.SeedBrokers(brokers...), kgo.ConsumeTopics(topic),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()))
	if err != nil {
		t.Fatal(err)
	}
	defer consumer.Close()
	var got []string
	for len(got) < 3 {
		fetches := consumer.PollFetches(ctx)
		if err := fetches.Err0(); err != nil {
			t.Fatalf("consume: %v", err)
		}
		fetches.EachRecord(func(r *kgo.Record) {
			got = append(got, string(r.Key)+"="+string(r.Value))
		})
	}
	want := []string{"key-0=payload-0", "key-1=payload-1", "key-2=payload-2"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("record %d = %q, want %q (order must match append order)", i, got[i], want[i])
		}
	}
}

// When the broker cannot be reached, the relay must fail and leave every event unpublished, so the next pass retries.
func TestBrokerOutageLeavesEventsUnpublished(t *testing.T) {
	brokers, pool := kafkaEnv(t)
	topic := newTopic(t, brokers)
	appendEvents(t, pool, topic, 2)

	dead, err := NewKafkaPublisher([]string{"127.0.0.1:1"})
	if err != nil {
		t.Fatal(err)
	}
	defer dead.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if n, err := Relay(ctx, pool, 100, dead); err == nil || n != 0 {
		t.Fatalf("relay to a dead broker: n=%d err=%v; want an error and zero", n, err)
	}
	if got := unpublished(t, pool); got != 2 {
		t.Fatalf("unpublished = %d after an outage, want 2", got)
	}
}
