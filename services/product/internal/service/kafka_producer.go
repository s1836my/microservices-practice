package service

import (
	"context"
	"fmt"

	"github.com/segmentio/kafka-go"
)

// EventPublisher publishes raw event payloads to a message broker.
type EventPublisher interface {
	Publish(ctx context.Context, eventType string, payload []byte) error
	Close() error
}

type kafkaPublisher struct {
	writer *kafka.Writer
}

// NewKafkaPublisher creates a Kafka-backed EventPublisher.
func NewKafkaPublisher(brokers []string, topic string) EventPublisher {
	w := &kafka.Writer{
		Addr:     kafka.TCP(brokers...),
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	}
	return &kafkaPublisher{writer: w}
}

func (p *kafkaPublisher) Publish(ctx context.Context, eventType string, payload []byte) error {
	err := p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(eventType),
		Value: payload,
	})
	if err != nil {
		return fmt.Errorf("kafka write message: %w", err)
	}
	return nil
}

func (p *kafkaPublisher) Close() error {
	return p.writer.Close()
}

// noopPublisher is used when Kafka is not configured (e.g., development).
type noopPublisher struct{}

func NewNoopPublisher() EventPublisher { return &noopPublisher{} }

func (p *noopPublisher) Publish(_ context.Context, _ string, _ []byte) error { return nil }
func (p *noopPublisher) Close() error                                         { return nil }
