package outbox

import (
	"context"
	"fmt"

	"github.com/twmb/franz-go/pkg/kgo"
)

// KafkaPublisher publishes with acks=all. Status: compiled, not yet run against a broker (VERIFY-KAFKA).
type KafkaPublisher struct {
	client *kgo.Client
}

// NewKafkaPublisher connects to the brokers.
func NewKafkaPublisher(brokers []string) (*KafkaPublisher, error) {
	c, err := kgo.NewClient(kgo.SeedBrokers(brokers...), kgo.RequiredAcks(kgo.AllISRAcks()))
	if err != nil {
		return nil, fmt.Errorf("kafka: client: %w", err)
	}
	return &KafkaPublisher{client: c}, nil
}

// Publish implements Publisher.
func (p *KafkaPublisher) Publish(ctx context.Context, topic string, key, value []byte) error {
	return p.client.ProduceSync(ctx, &kgo.Record{Topic: topic, Key: key, Value: value}).FirstErr()
}

// Close flushes and closes the client.
func (p *KafkaPublisher) Close() { p.client.Close() }
