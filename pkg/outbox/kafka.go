package outbox

import (
	"context"
	"fmt"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

// DeliveryTimeout is how long a record may take to be acknowledged by enough replicas before Publish fails. The event
// then stays unpublished and the next relay pass sends it again.
var DeliveryTimeout = 20 * time.Second

// KafkaPublisher publishes with acks=all and a delivery deadline. Verified on one broker and on three, with one and two
// of them killed (kafka_cluster_test.go).
type KafkaPublisher struct {
	client *kgo.Client
}

// NewKafkaPublisher connects to the brokers.
func NewKafkaPublisher(brokers []string) (*KafkaPublisher, error) {
	c, err := kgo.NewClient(kgo.SeedBrokers(brokers...), kgo.RequiredAcks(kgo.AllISRAcks()),
		// Without a deadline the client retries a record for ever, and the context does not stop it once the record
		// is on the wire: with no quorum the relay would hang inside its database transaction. Measured on a
		// three-broker cluster with two brokers down (pkg/outbox/kafka_cluster_test.go).
		kgo.RecordDeliveryTimeout(DeliveryTimeout))
	if err != nil {
		return nil, fmt.Errorf("kafka: client: %w", err)
	}
	return &KafkaPublisher{client: c}, nil
}

// Publish implements Publisher.
func (p *KafkaPublisher) Publish(ctx context.Context, topic string, key, value []byte) error {
	// Publish returns when the record is acknowledged, or when ctx ends, whichever is first. The client cannot be
	// trusted to stop on its own: with the brokers' controller majority gone it holds a record waiting for a leader,
	// past both ctx and its delivery timeout (measured). A record abandoned here may still be delivered later; the
	// caller has not marked it published, so it is sent again, and consumers dedupe by event id.
	done := make(chan error, 1)
	p.client.Produce(ctx, &kgo.Record{Topic: topic, Key: key, Value: value}, func(_ *kgo.Record, err error) { done <- err })
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return fmt.Errorf("kafka: not acknowledged before the deadline: %w", ctx.Err())
	}
}

// Close flushes and closes the client.
func (p *KafkaPublisher) Close() { p.client.Close() }
