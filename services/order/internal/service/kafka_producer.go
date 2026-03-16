package service

import (
	"context"
	"fmt"

	"github.com/segmentio/kafka-go"
)

type EventPublisher interface {
	Publish(ctx context.Context, eventType string, payload []byte) error
	Close() error
}

type kafkaPublisher struct {
	writer *kafka.Writer
}

func NewKafkaPublisher(brokers []string, topic string) EventPublisher {
	return &kafkaPublisher{
		writer: &kafka.Writer{
			Addr:     kafka.TCP(brokers...),
			Topic:    topic,
			Balancer: &kafka.LeastBytes{},
		},
	}
}

func (p *kafkaPublisher) Publish(ctx context.Context, eventType string, payload []byte) error {
	if err := p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(eventType),
		Value: payload,
	}); err != nil {
		return fmt.Errorf("kafka write message: %w", err)
	}
	return nil
}

func (p *kafkaPublisher) Close() error {
	return p.writer.Close()
}

type noopPublisher struct{}

func NewNoopPublisher() EventPublisher { return &noopPublisher{} }

func (p *noopPublisher) Publish(_ context.Context, _ string, _ []byte) error { return nil }
func (p *noopPublisher) Close() error                                        { return nil }
