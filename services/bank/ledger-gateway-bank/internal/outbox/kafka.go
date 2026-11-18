package outbox

import (
	"context"
	"fmt"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

// KafkaPublisher publishes with acks=all and idempotent producing enabled.
//
// Each record must be acknowledged within 20 seconds, or Publish fails and the next relay pass sends it again.
type KafkaPublisher struct {
	client *kgo.Client
}

// NewKafkaPublisher connects to the brokers.
func NewKafkaPublisher(brokers []string) (*KafkaPublisher, error) {
	c, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.RequiredAcks(kgo.AllISRAcks()),
		// A deadline per record: without one the client retries for ever and the relay hangs with no quorum.
		kgo.RecordDeliveryTimeout(20*time.Second),
		kgo.ProducerBatchCompression(kgo.ZstdCompression(), kgo.SnappyCompression(), kgo.NoCompression()),
	)
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
