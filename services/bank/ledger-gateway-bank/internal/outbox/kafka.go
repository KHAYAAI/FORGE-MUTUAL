package outbox

import (
	"context"
	"fmt"

	"github.com/twmb/franz-go/pkg/kgo"
)

// KafkaPublisher publishes with acks=all and idempotent producing enabled.
//
// Status: compiled against franz-go v1.18.1; not yet run against a broker in this environment (VERIFY-KAFKA).
type KafkaPublisher struct {
	client *kgo.Client
}

// NewKafkaPublisher connects to the brokers.
func NewKafkaPublisher(brokers []string) (*KafkaPublisher, error) {
	c, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.ProducerBatchCompression(kgo.ZstdCompression(), kgo.SnappyCompression(), kgo.NoCompression()),
	)
	if err != nil {
		return nil, fmt.Errorf("kafka: client: %w", err)
	}
	return &KafkaPublisher{client: c}, nil
}

// Publish implements Publisher.
func (p *KafkaPublisher) Publish(ctx context.Context, topic string, key, value []byte) error {
	res := p.client.ProduceSync(ctx, &kgo.Record{Topic: topic, Key: key, Value: value})
	return res.FirstErr()
}

// Close flushes and closes the client.
func (p *KafkaPublisher) Close() { p.client.Close() }
