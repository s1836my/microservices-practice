package service

import (
	"context"
	"errors"
	"log/slog"

	"github.com/segmentio/kafka-go"
)

// ProductConsumer consumes product.events and updates the search index.
type ProductConsumer struct {
	reader    *kafka.Reader
	processor *ProductEventProcessor
	log       *slog.Logger
}

// NewProductConsumer creates a new Kafka-backed consumer.
func NewProductConsumer(brokers []string, groupID, topic string, processor *ProductEventProcessor, log *slog.Logger) *ProductConsumer {
	return &ProductConsumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers: brokers,
			GroupID: groupID,
			Topic:   topic,
		}),
		processor: processor,
		log:       log,
	}
}

// Run starts consuming until ctx is canceled.
func (c *ProductConsumer) Run(ctx context.Context) {
	defer c.reader.Close()

	for {
		msg, err := c.reader.ReadMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			if c.log != nil {
				c.log.Error("failed to read product event", "error", err)
			}
			continue
		}

		if err := c.processor.HandleMessage(ctx, msg.Value); err != nil && c.log != nil {
			c.log.Error("failed to process product event", "error", err, "offset", msg.Offset)
		}
	}
}
