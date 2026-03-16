package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apperrors "github.com/yourorg/micromart/pkg/errors"
	"github.com/yourorg/micromart/services/order/internal/model"
	"github.com/yourorg/micromart/services/order/internal/service"
)

type mockOrderRepo struct {
	orders map[uuid.UUID]*model.Order
	outbox []*model.OutboxEvent
	total  int32
}

func newMockOrderRepo() *mockOrderRepo {
	return &mockOrderRepo{
		orders: make(map[uuid.UUID]*model.Order),
	}
}

func (m *mockOrderRepo) Create(_ context.Context, order *model.Order, _ *model.SagaState, outbox *model.OutboxEvent) (*model.Order, error) {
	for _, existing := range m.orders {
		if existing.IdempotencyKey == order.IdempotencyKey {
			return nil, apperrors.NewAlreadyExists("idempotency key already exists")
		}
	}
	created := *order
	created.CreatedAt = time.Now()
	created.UpdatedAt = created.CreatedAt
	m.orders[created.ID] = &created
	m.outbox = append(m.outbox, outbox)
	return &created, nil
}

func (m *mockOrderRepo) FindByID(_ context.Context, id uuid.UUID) (*model.Order, error) {
	order, ok := m.orders[id]
	if !ok {
		return nil, apperrors.NewNotFound("order not found")
	}
	copied := *order
	copied.Items = append([]model.OrderItem(nil), order.Items...)
	return &copied, nil
}

func (m *mockOrderRepo) ListByUser(_ context.Context, userID uuid.UUID, _, _ int32) ([]*model.Order, int32, error) {
	var orders []*model.Order
	for _, order := range m.orders {
		if order.UserID == userID {
			copied := *order
			copied.Items = append([]model.OrderItem(nil), order.Items...)
			orders = append(orders, &copied)
		}
	}
	return orders, int32(len(orders)), nil
}

func (m *mockOrderRepo) Cancel(_ context.Context, id uuid.UUID, reason string, outbox *model.OutboxEvent) error {
	order, ok := m.orders[id]
	if !ok {
		return apperrors.NewNotFound("order not found")
	}
	if order.Status != model.OrderStatusCreated && order.Status != model.OrderStatusPaymentPending {
		return apperrors.NewInvalidInput("order cannot be cancelled in current state")
	}
	order.Status = model.OrderStatusCancelled
	order.FailureReason = reason
	order.UpdatedAt = time.Now()
	m.outbox = append(m.outbox, outbox)
	return nil
}

func (m *mockOrderRepo) ListUnpublishedEvents(_ context.Context, _ int) ([]*model.OutboxEvent, error) {
	var events []*model.OutboxEvent
	for _, event := range m.outbox {
		if !event.Published {
			events = append(events, event)
		}
	}
	return events, nil
}

func (m *mockOrderRepo) MarkEventPublished(_ context.Context, id uuid.UUID) error {
	for _, event := range m.outbox {
		if event.ID == id {
			event.Published = true
			now := time.Now()
			event.PublishedAt = &now
			return nil
		}
	}
	return apperrors.NewNotFound("outbox event not found")
}

func TestOrderService_Create_Success(t *testing.T) {
	repo := newMockOrderRepo()
	svc := service.NewOrderService(repo)

	userID := uuid.New()
	productID := uuid.New()
	sellerID := uuid.New()

	order, err := svc.Create(context.Background(), service.CreateOrderInput{
		UserID:         userID,
		IdempotencyKey: "idem-1",
		Items: []service.CreateItemInput{
			{ProductID: productID, ProductName: "Keyboard", SellerID: sellerID, UnitPrice: 5000, Quantity: 2},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, userID, order.UserID)
	assert.Equal(t, model.OrderStatusCreated, order.Status)
	assert.Equal(t, int64(10000), order.TotalAmount)
	require.Len(t, order.Items, 1)
	assert.Equal(t, int64(10000), order.Items[0].Subtotal)
	require.Len(t, repo.outbox, 1)
	assert.Equal(t, "order.created", repo.outbox[0].EventType)
}

func TestOrderService_Create_InvalidItem(t *testing.T) {
	repo := newMockOrderRepo()
	svc := service.NewOrderService(repo)

	_, err := svc.Create(context.Background(), service.CreateOrderInput{
		UserID:         uuid.New(),
		IdempotencyKey: "idem-1",
		Items: []service.CreateItemInput{
			{ProductID: uuid.New(), UnitPrice: 1000, Quantity: 0},
		},
	})
	require.Error(t, err)
	var appErr *apperrors.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeInvalidInput, appErr.Code)
}

func TestOrderService_Get_PermissionDenied(t *testing.T) {
	repo := newMockOrderRepo()
	orderID := uuid.New()
	repo.orders[orderID] = &model.Order{
		ID:       orderID,
		UserID:   uuid.New(),
		Status:   model.OrderStatusCreated,
		Currency: "JPY",
	}
	svc := service.NewOrderService(repo)

	_, err := svc.Get(context.Background(), orderID, uuid.New())
	require.Error(t, err)
	var appErr *apperrors.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodePermissionDenied, appErr.Code)
}

func TestOrderService_List_DefaultPagination(t *testing.T) {
	repo := newMockOrderRepo()
	userID := uuid.New()
	repo.orders[uuid.New()] = &model.Order{ID: uuid.New(), UserID: userID, Status: model.OrderStatusCreated, Currency: "JPY"}
	svc := service.NewOrderService(repo)

	orders, total, err := svc.List(context.Background(), service.ListOrdersInput{UserID: userID})
	require.NoError(t, err)
	assert.Len(t, orders, 1)
	assert.Equal(t, int32(1), total)
}

func TestOrderService_Cancel_Success(t *testing.T) {
	repo := newMockOrderRepo()
	orderID := uuid.New()
	userID := uuid.New()
	repo.orders[orderID] = &model.Order{
		ID:       orderID,
		UserID:   userID,
		Status:   model.OrderStatusCreated,
		Currency: "JPY",
		Items:    []model.OrderItem{{ProductID: uuid.New(), Quantity: 1}},
	}
	svc := service.NewOrderService(repo)

	err := svc.Cancel(context.Background(), orderID, userID, "changed_mind")
	require.NoError(t, err)
	assert.Equal(t, model.OrderStatusCancelled, repo.orders[orderID].Status)
	assert.Equal(t, "changed_mind", repo.orders[orderID].FailureReason)
	require.Len(t, repo.outbox, 1)
	assert.Equal(t, "order.cancelled", repo.outbox[0].EventType)
}

func TestOrderService_Cancel_InvalidState(t *testing.T) {
	repo := newMockOrderRepo()
	orderID := uuid.New()
	userID := uuid.New()
	repo.orders[orderID] = &model.Order{
		ID:       orderID,
		UserID:   userID,
		Status:   model.OrderStatusCompleted,
		Currency: "JPY",
	}
	svc := service.NewOrderService(repo)

	err := svc.Cancel(context.Background(), orderID, userID, "too_late")
	require.Error(t, err)
	var appErr *apperrors.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeInvalidInput, appErr.Code)
}
