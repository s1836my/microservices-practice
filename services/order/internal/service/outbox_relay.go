package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/yourorg/micromart/services/order/internal/repository"
)

const (
	outboxBatchSize    = 50
	outboxPollInterval = 5 * time.Second
)

type OutboxRelay struct {
	repo      repository.OrderRepository
	publisher EventPublisher
	log       *slog.Logger
}

func NewOutboxRelay(repo repository.OrderRepository, publisher EventPublisher, log *slog.Logger) *OutboxRelay {
	return &OutboxRelay{repo: repo, publisher: publisher, log: log}
}

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
