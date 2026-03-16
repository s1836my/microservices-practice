package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/yourorg/micromart/services/order/internal/model"
)

type OrderRepository interface {
	Create(ctx context.Context, order *model.Order, saga *model.SagaState, outbox *model.OutboxEvent) (*model.Order, error)
	FindByID(ctx context.Context, id uuid.UUID) (*model.Order, error)
	ListByUser(ctx context.Context, userID uuid.UUID, page, pageSize int32) ([]*model.Order, int32, error)
	Cancel(ctx context.Context, id uuid.UUID, reason string, outbox *model.OutboxEvent) error
	ListUnpublishedEvents(ctx context.Context, limit int) ([]*model.OutboxEvent, error)
	MarkEventPublished(ctx context.Context, id uuid.UUID) error
}
