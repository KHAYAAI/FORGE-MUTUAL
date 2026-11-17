package outbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/khayaai/forge-mutual/pkg/uuid"
)

// TestClusterLosesABrokerAndNoEventIsLostOrReordered runs the outbox relay against three brokers while they fail.
//
//  1. While events are being relayed, one broker is killed. Every event must still arrive, and the events of each key
//     must arrive in the order they were appended. Leadership moves; nothing is lost.
//  2. A second broker is killed. With one of three left, a write cannot reach two in-sync replicas, so the relay must
//     fail and leave the events unpublished, never mark them published without the cluster having them.
//  3. Both are started again. The next relay pass delivers what was held back.
//
// Needs KAFKA_CLUSTER_BROKERS, KAFKA_CLUSTER_CONTAINERS (scripts/kafka-cluster.sh) and OUTBOX_TEST_DSN.
func TestClusterLosesABrokerAndNoEventIsLostOrReordered(t *testing.T) {
	brokersEnv, containersEnv := os.Getenv("KAFKA_CLUSTER_BROKERS"), os.Getenv("KAFKA_CLUSTER_CONTAINERS")
	if brokersEnv == "" || containersEnv == "" {
		t.Skip("KAFKA_CLUSTER_BROKERS and KAFKA_CLUSTER_CONTAINERS are not set")
	}
	t.Setenv("KAFKA_TEST_BROKERS", brokersEnv)
	brokers, pool := kafkaEnv(t)
	names := strings.Split(containersEnv, ",")
	docker := func(args ...string) {
		t.Helper()
		if out, err := exec.Command("docker", args...).CombinedOutput(); err != nil {
			t.Fatalf("docker %v: %v: %s", args, err, out)
		}
	}
	t.Cleanup(func() { _ = exec.Command("docker", append([]string{"start"}, names...)...).Run() })

	admin, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	topic := fmt.Sprintf("forge.test.cluster.%d", time.Now().UnixNano())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	min2 := "2"
	resp, err := kadm.NewClient(admin).CreateTopic(ctx, 3, 3, map[string]*string{"min.insync.replicas": &min2}, topic)
	if err != nil || resp.Err != nil {
		t.Fatalf("create topic: %v %v", err, resp.Err)
	}

	const keys, perKey = 6, 60
	for i := 0; i < perKey; i++ {
		for k := 0; k < keys; k++ {
			appendKeyed(t, pool, topic, fmt.Sprintf("acct-%d", k), i)
		}
	}
	saved := DeliveryTimeout
	DeliveryTimeout = 5 * time.Second // shorter than each relay pass's own deadline, so a refusal is seen in the pass
	t.Cleanup(func() { DeliveryTimeout = saved })
	pub, err := NewKafkaPublisher(brokers)
	if err != nil {
		t.Fatal(err)
	}
	defer pub.Close()

	// 1. Relay in small passes, as the service does, and kill a broker part-way through.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(300 * time.Millisecond)
		docker("kill", names[0])
	}()
	deadline := time.Now().Add(2 * time.Minute)
	for unpublished(t, pool) > 0 {
		if time.Now().After(deadline) {
			t.Fatalf("%d events still unpublished two minutes after one broker of three died", unpublished(t, pool))
		}
		pass, cancelPass := context.WithTimeout(ctx, 10*time.Second)
		_, _ = Relay(pass, pool, 20, pub) // a pass cut off by a leader change is retried by the next one
		cancelPass()
	}
	wg.Wait()
	checkDelivered(t, ctx, brokers[1:], topic, keys, perKey)

	// 2. A second broker down: no write can reach two in-sync replicas.
	docker("kill", names[1])
	for k := 0; k < keys; k++ {
		appendKeyed(t, pool, topic, fmt.Sprintf("acct-%d", k), perKey)
	}
	pass, cancelPass := context.WithTimeout(ctx, 15*time.Second)
	n, err := Relay(pass, pool, 100, pub)
	cancelPass()
	if err == nil {
		t.Fatalf("relay with one broker of three succeeded (%d events): acks=all with min.insync.replicas=2 must refuse", n)
	}
	if got := unpublished(t, pool); n != 0 || got != keys {
		t.Fatalf("after a refused write: %d relayed, %d unpublished; want 0 and %d", n, got, keys)
	}

	// 3. Back to three: what was held back is delivered.
	docker("start", names[0], names[1])
	deadline = time.Now().Add(2 * time.Minute)
	for unpublished(t, pool) > 0 {
		if time.Now().After(deadline) {
			t.Fatalf("%d events still unpublished two minutes after the brokers came back", unpublished(t, pool))
		}
		pass, cancelPass := context.WithTimeout(ctx, 10*time.Second)
		_, _ = Relay(pass, pool, 100, pub)
		cancelPass()
	}
	checkDelivered(t, ctx, brokers, topic, keys, perKey+1)
}

func appendKeyed(t *testing.T, pool *pgxpool.Pool, topic, key string, seq int) {
	t.Helper()
	id, err := uuid.NewV4()
	if err != nil {
		t.Fatal(err)
	}
	e := Event{EventID: id, Topic: topic, Key: []byte(key), Payload: []byte(fmt.Sprint(seq))}
	if err := pgx.BeginFunc(context.Background(), pool, func(tx pgx.Tx) error { return Append(context.Background(), tx, e) }); err != nil {
		t.Fatal(err)
	}
}

// checkDelivered reads the whole topic and requires every key's events 0..perKey-1, each at least once, in order. A
// relay pass cut off after the broker took a batch but before the outbox row was marked sends that batch again, so a
// repeat of the latest event is allowed (consumers dedupe by event id); going backwards, or missing one, is not.
func checkDelivered(t *testing.T, ctx context.Context, brokers []string, topic string, keys, perKey int) {
	t.Helper()
	c, err := kgo.NewClient(kgo.SeedBrokers(brokers...), kgo.ConsumeTopics(topic), kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	next := map[string]int{}
	repeats := 0
	done := func() bool {
		for k := 0; k < keys; k++ {
			if next[fmt.Sprintf("acct-%d", k)] < perKey {
				return false
			}
		}
		return true
	}
	pollCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	for !done() {
		f := c.PollFetches(pollCtx)
		if pollCtx.Err() != nil {
			t.Fatalf("timed out reading the topic; got %v", next)
		}
		f.EachRecord(func(r *kgo.Record) {
			key := string(r.Key)
			var seq int
			fmt.Sscan(string(r.Value), &seq)
			switch {
			case seq == next[key]:
				next[key]++
			case seq == next[key]-1:
				repeats++
			default:
				t.Fatalf("key %s: got event %d after %d: lost or reordered", key, seq, next[key]-1)
			}
		})
	}
	t.Logf("all %d keys complete and in order; %d repeated deliveries", keys, repeats)
}
