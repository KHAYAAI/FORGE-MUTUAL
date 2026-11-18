package feed

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	eventsv1 "github.com/khayaai/forge-mutual/pkg/api/forge/events/v1"
	"github.com/khayaai/forge-mutual/pkg/eventbus"
	"github.com/khayaai/forge-mutual/pkg/events"
	"github.com/khayaai/forge-mutual/pkg/outbox"
	"github.com/khayaai/forge-mutual/services/shared/compliance-engine/internal/monitor"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
)

// The whole path, through a real broker: payments publish settled payments to Kafka, the compliance consumer reads them
// through the event bus, and a case opens in the database. Needs COMPLIANCE_TEST_DSN and KAFKA_TEST_BROKERS.

func suffix(t *testing.T) string {
	t.Helper()
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}

// exists reports whether a topic-creation error only says the topic is already there, which a re-run expects.
func exists(err error) bool {
	return errors.Is(err, kerr.TopicAlreadyExists) || strings.Contains(err.Error(), "already exists")
}

func createTopics(t *testing.T, brokers string, topics ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cl, err := kgo.NewClient(kgo.SeedBrokers(strings.Split(brokers, ",")...))
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()
	for _, topic := range topics {
		resp, err := kadm.NewClient(cl).CreateTopic(ctx, 1, 1, nil, topic)
		if err != nil && !exists(err) {
			t.Fatal(err)
		}
		if resp.Err != nil && !exists(resp.Err) {
			t.Fatal(resp.Err)
		}
	}
}

func TestSettledPaymentsOnTheBusOpenACase(t *testing.T) {
	brokers := os.Getenv("KAFKA_TEST_BROKERS")
	if brokers == "" {
		t.Skip("KAFKA_TEST_BROKERS not set")
	}
	pool := openPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// Topics are created first. A consumer does not create topics, and a group on a missing topic is never assigned
	// partitions, so the consumer would never report ready.
	createTopics(t, brokers, TopicPaymentSettled, TopicCreditBooked)

	name := "compliance-feed-" + suffix(t)
	ready := make(chan struct{})
	consumer := &eventbus.Consumer{
		Name: name, Pool: pool, Brokers: strings.Split(brokers, ","), Group: name,
		Topics: []string{TopicPaymentSettled, TopicCreditBooked},
		Handle: (&Feed{Cfg: monitor.DefaultConfig(), Now: time.Now}).Handle,
		Log:    slog.Default(), Ready: ready,
	}
	if err := eventbus.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	runErr := make(chan error, 1)
	go func() { runErr <- consumer.Run(ctx) }()
	select {
	case <-ready:
	case err := <-runErr:
		t.Fatalf("the consumer stopped before it joined its group: %v", err)
	case <-ctx.Done():
		t.Fatal("the consumer never joined its group")
	}

	pub, err := outbox.NewKafkaPublisher(strings.Split(brokers, ","))
	if err != nil {
		t.Fatal(err)
	}
	defer pub.Close()

	party := "33333333-3333-4333-8333-" + suffix(t) + suffix(t)[:4]
	for i := 0; i < 3; i++ {
		_, payload, err := events.Wrap(events.Params{Type: TopicPaymentSettled, Entity: "BANK", Producer: "payments-orchestrator"},
			&eventsv1.PaymentEvent{PartyId: party, AmountMinor: "2000000", Asset: "ZAR", Status: "SETTLED", BeneficiaryKey: "payee-k"})
		if err != nil {
			t.Fatal(err)
		}
		if err := pub.Publish(ctx, TopicPaymentSettled, []byte(party), payload); err != nil {
			t.Fatal(err)
		}
	}

	for {
		if contains(openCases(t, pool, party), monitor.RuleVelocity) {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal("no velocity case opened from the events on the bus")
		case <-time.After(500 * time.Millisecond):
		}
	}
}
