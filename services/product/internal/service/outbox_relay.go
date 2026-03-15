package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/yourorg/micromart/services/product/internal/repository"
)

const (
	outboxBatchSize    = 50
	outboxPollInterval = 5 * time.Second
)

// OutboxRelay polls the product_outbox table and publishes unpublished events to Kafka.
type OutboxRelay struct {
	repo      repository.ProductRepository
	publisher EventPublisher
	log       *slog.Logger
}

func NewOutboxRelay(repo repository.ProductRepository, publisher EventPublisher, log *slog.Logger) *OutboxRelay {
	return &OutboxRelay{repo: repo, publisher: publisher, log: log}
}

// Run starts the relay loop. It blocks until ctx is cancelled.
func (r *OutboxRelay) Run(ctx context.Context) {
	ticker := time.NewTicker(outboxPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.processBatch(ctx)
		}
	}
}

// RunOnce processes a single batch of unpublished events and returns.
// Useful for testing and for one-shot invocations.
func (r *OutboxRelay) RunOnce(ctx context.Context) {
	r.processBatch(ctx)
}

func (r *OutboxRelay) processBatch(ctx context.Context) {
	events, err := r.repo.ListUnpublishedEvents(ctx, outboxBatchSize)
	if err != nil {
		r.log.Error("outbox relay: list unpublished events", "error", err)
		return
	}

	for _, event := range events {
		if err := r.publisher.Publish(ctx, event.EventType, event.Payload); err != nil {
			r.log.Error("outbox relay: publish event", "event_id", event.ID, "event_type", event.EventType, "error", err)
			continue
		}
		if err := r.repo.MarkEventPublished(ctx, event.ID); err != nil {
			r.log.Error("outbox relay: mark published", "event_id", event.ID, "error", err)
		}
	}
}
