package eventbus

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"

	eventsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/events/v1"
	"github.com/khayaai/forge-mutual/pkg/events"
)

// These tests need a real broker (KAFKA_TEST_BROKERS) and a throwaway database (EVENTBUS_TEST_DSN). Each test uses its
// own topic and consumer names.
func env(t *testing.T) ([]string, *pgxpool.Pool) {
	t.Helper()
	brokers := os.Getenv("KAFKA_TEST_BROKERS")
	dsn := os.Getenv("EVENTBUS_TEST_DSN")
	if brokers == "" || dsn == "" {
		t.Skip("KAFKA_TEST_BROKERS and EVENTBUS_TEST_DSN are not set")
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
	return strings.Split(brokers, ","), pool
}

func topic(t *testing.T, brokers []string) string {
	t.Helper()
	name := fmt.Sprintf("forge.test.eventbus.%d", time.Now().UnixNano())
	cl, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cl.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, err := kadm.NewClient(cl).CreateTopic(ctx, 1, 1, nil, name)
	if err != nil || resp.Err != nil {
		t.Fatalf("create topic: %v %v", err, resp.Err)
	}
	return name
}

func produce(t *testing.T, brokers []string, topic string, values ...[]byte) {
	t.Helper()
	cl, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()
	var recs []*kgo.Record
	for _, v := range values {
		recs = append(recs, &kgo.Record{Topic: topic, Key: []byte("k"), Value: v})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := cl.ProduceSync(ctx, recs...).FirstErr(); err != nil {
		t.Fatal(err)
	}
}

func envelope(t *testing.T, topic, id string) []byte {
	t.Helper()
	_, data, err := events.Wrap(events.Params{EventID: id, Type: topic, Entity: "BANK", Producer: "test"}, &eventsv1.PaymentEvent{PaymentId: id})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// The applied table is the test's business state: each handled event writes one row through the transaction.
func applied(t *testing.T, pool *pgxpool.Pool, consumer string) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), `select event_id from eventbus_processed where consumer = $1 order by event_id`, consumer)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		_ = rows.Scan(&id)
		out = append(out, id)
	}
	return out
}

// run starts a consumer and stops it once wait() returns true or the deadline passes.
func run(t *testing.T, c *Consumer, wait func() bool) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errc := make(chan error, 1)
	go func() { errc <- c.Run(ctx) }()
	deadline := time.Now().Add(60 * time.Second)
	for !wait() {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for the consumer")
		}
		time.Sleep(100 * time.Millisecond)
	}
	cancel()
	if err := <-errc; err != nil {
		t.Fatalf("consumer: %v", err)
	}
}

// Every event is applied once. A second group reading the same topic from the start must apply nothing again,
// because the processed table is keyed by consumer name and event id.
func TestEachEventAppliedOnceEvenWhenRedelivered(t *testing.T) {
	brokers, pool := env(t)
	tp := topic(t, brokers)
	produce(t, brokers, tp, envelope(t, tp, "e1"), envelope(t, tp, "e2"), envelope(t, tp, "e3"))

	var mu sync.Mutex
	calls := 0
	h := func(ctx context.Context, tx pgx.Tx, env *eventsv1.EventEnvelope) error {
		mu.Lock()
		calls++
		mu.Unlock()
		_, err := tx.Exec(ctx, `insert into eventbus_applied_test (event_id) values ($1)`, env.EventId)
		return err
	}
	if _, err := pool.Exec(context.Background(), `create table if not exists eventbus_applied_test (event_id text primary key)`); err != nil {
		t.Fatal(err)
	}
	_, _ = pool.Exec(context.Background(), `truncate eventbus_applied_test`)

	name := "orders-" + tp
	c := &Consumer{Name: name, Pool: pool, Brokers: brokers, Group: "g1-" + tp, Topics: []string{tp}, Handle: h}
	run(t, c, func() bool { return len(applied(t, pool, name)) == 3 })

	mu.Lock()
	first := calls
	mu.Unlock()
	// A new group reads from the start again. The processed table must stop every handler call.
	c2 := &Consumer{Name: name, Pool: pool, Brokers: brokers, Group: "g2-" + tp, Topics: []string{tp}, Handle: h}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	_ = c2.Run(ctx)

	mu.Lock()
	defer mu.Unlock()
	if first != 3 || calls != 3 {
		t.Fatalf("handler calls = %d after first group and %d after redelivery; want 3 and 3", first, calls)
	}
	var n int
	_ = pool.QueryRow(context.Background(), `select count(*) from eventbus_applied_test`).Scan(&n)
	if n != 3 {
		t.Fatalf("applied rows = %d, want 3", n)
	}
}

// A message that is not an envelope is dead-lettered, and the good event after it is still applied.
func TestUndecodableMessageIsDeadLettered(t *testing.T) {
	brokers, pool := env(t)
	tp := topic(t, brokers)
	produce(t, brokers, tp, []byte{0xff, 0x00, 0x13}, envelope(t, tp, "after-garbage"))

	name := "dl-" + tp
	c := &Consumer{Name: name, Pool: pool, Brokers: brokers, Group: "g-" + tp, Topics: []string{tp},
		Handle: func(context.Context, pgx.Tx, *eventsv1.EventEnvelope) error { return nil }}
	run(t, c, func() bool { return len(applied(t, pool, name)) == 1 })

	var dead int
	_ = pool.QueryRow(context.Background(), `select count(*) from eventbus_dead_letter where consumer = $1`, name).Scan(&dead)
	if dead != 1 {
		t.Fatalf("dead letters = %d, want 1", dead)
	}
}

// A handler that keeps failing is retried and then dead-lettered, so the partition moves on.
func TestPersistentFailureIsDeadLettered(t *testing.T) {
	brokers, pool := env(t)
	tp := topic(t, brokers)
	produce(t, brokers, tp, envelope(t, tp, "bad"), envelope(t, tp, "good"))

	name := "fail-" + tp
	attempts := 0
	c := &Consumer{Name: name, Pool: pool, Brokers: brokers, Group: "g-" + tp, Topics: []string{tp}, Attempts: 3, Backoff: 10 * time.Millisecond,
		Handle: func(_ context.Context, _ pgx.Tx, env *eventsv1.EventEnvelope) error {
			if env.EventId == "bad" {
				attempts++
				return errors.New("downstream rejects this event")
			}
			return nil
		}}
	run(t, c, func() bool { return len(applied(t, pool, name)) == 1 })

	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
	var dead int
	_ = pool.QueryRow(context.Background(), `select count(*) from eventbus_dead_letter where consumer = $1 and error like '%rejects%'`, name).Scan(&dead)
	if dead != 1 {
		t.Fatalf("dead letters = %d, want 1", dead)
	}
}
