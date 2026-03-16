package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	apperrors "github.com/yourorg/micromart/pkg/errors"
	"github.com/yourorg/micromart/services/order/internal/model"
	"github.com/yourorg/micromart/services/order/internal/repository"
	"github.com/yourorg/micromart/services/order/internal/validator"
)

type CreateItemInput struct {
	ProductID   uuid.UUID
	ProductName string
	SellerID    uuid.UUID
	UnitPrice   int64
	Quantity    int32
}

type CreateOrderInput struct {
	UserID         uuid.UUID
	IdempotencyKey string
	Items          []CreateItemInput
}

type ListOrdersInput struct {
	UserID   uuid.UUID
	Page     int32
	PageSize int32
}

type OrderService interface {
	Create(ctx context.Context, in CreateOrderInput) (*model.Order, error)
	Get(ctx context.Context, orderID, userID uuid.UUID) (*model.Order, error)
	List(ctx context.Context, in ListOrdersInput) ([]*model.Order, int32, error)
	Cancel(ctx context.Context, orderID, userID uuid.UUID, reason string) error
}

type orderService struct {
	repo repository.OrderRepository
}

func NewOrderService(repo repository.OrderRepository) OrderService {
	return &orderService{repo: repo}
}

func (s *orderService) Create(ctx context.Context, in CreateOrderInput) (*model.Order, error) {
	if in.UserID == uuid.Nil {
		return nil, apperrors.NewInvalidInput("user_id is required")
	}
	if in.IdempotencyKey == "" {
		return nil, apperrors.NewInvalidInput("idempotency_key is required")
	}
	if len(in.Items) == 0 {
		return nil, apperrors.NewInvalidInput("at least one item is required")
	}

	order := &model.Order{
		ID:             uuid.New(),
		UserID:         in.UserID,
		Status:         model.OrderStatusCreated,
		Currency:       "JPY",
		IdempotencyKey: in.IdempotencyKey,
		Items:          make([]model.OrderItem, 0, len(in.Items)),
	}

	for idx, item := range in.Items {
		if item.ProductID == uuid.Nil {
			return nil, apperrors.NewInvalidInput(fmt.Sprintf("items[%d].product_id is required", idx))
		}
		if item.UnitPrice < 0 {
			return nil, apperrors.NewInvalidInput(fmt.Sprintf("items[%d].unit_price must be >= 0", idx))
		}
		if item.Quantity <= 0 {
			return nil, apperrors.NewInvalidInput(fmt.Sprintf("items[%d].quantity must be > 0", idx))
		}

		subtotal := item.UnitPrice * int64(item.Quantity)
		order.TotalAmount += subtotal
		productName := item.ProductName
		if productName == "" {
			productName = item.ProductID.String()
		}
		order.Items = append(order.Items, model.OrderItem{
			ID:          uuid.New(),
			ProductID:   item.ProductID,
			ProductName: productName,
			SellerID:    item.SellerID,
			UnitPrice:   item.UnitPrice,
			Quantity:    item.Quantity,
			Subtotal:    subtotal,
		})
	}

	saga := &model.SagaState{
		OrderID:         order.ID,
		PaymentStatus:   model.SagaProgressPending,
		InventoryStatus: model.SagaProgressPending,
		LastEventType:   "order.created",
	}

	outbox, err := repository.BuildOrderCreatedOutbox(order)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CodeInternal, err, "build order.created event")
	}

	return s.repo.Create(ctx, order, saga, outbox)
}

func (s *orderService) Get(ctx context.Context, orderID, userID uuid.UUID) (*model.Order, error) {
	order, err := s.repo.FindByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order.UserID != userID {
		return nil, apperrors.NewPermissionDenied("order does not belong to user")
	}
	return order, nil
}

func (s *orderService) List(ctx context.Context, in ListOrdersInput) ([]*model.Order, int32, error) {
	if in.UserID == uuid.Nil {
		return nil, 0, apperrors.NewInvalidInput("user_id is required")
	}
	return s.repo.ListByUser(ctx, in.UserID, validator.ValidatePage(in.Page), validator.ValidatePageSize(in.PageSize))
}

func (s *orderService) Cancel(ctx context.Context, orderID, userID uuid.UUID, reason string) error {
	if reason == "" {
		reason = "cancelled_by_user"
	}

	order, err := s.repo.FindByID(ctx, orderID)
	if err != nil {
		return err
	}
	if order.UserID != userID {
		return apperrors.NewPermissionDenied("order does not belong to user")
	}

	outbox, err := repository.BuildOrderCancelledOutbox(order, reason)
	if err != nil {
		return apperrors.Wrap(apperrors.CodeInternal, err, "build order.cancelled event")
	}
	return s.repo.Cancel(ctx, orderID, reason, outbox)
}
