package handler_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apperrors "github.com/yourorg/micromart/pkg/errors"
	orderv1 "github.com/yourorg/micromart/proto/order/v1"
	"github.com/yourorg/micromart/services/order/internal/handler"
	"github.com/yourorg/micromart/services/order/internal/model"
	"github.com/yourorg/micromart/services/order/internal/service"
)

type mockOrderService struct {
	createFn func(context.Context, service.CreateOrderInput) (*model.Order, error)
	getFn    func(context.Context, uuid.UUID, uuid.UUID) (*model.Order, error)
	listFn   func(context.Context, service.ListOrdersInput) ([]*model.Order, int32, error)
	cancelFn func(context.Context, uuid.UUID, uuid.UUID, string) error
}

func (m *mockOrderService) Create(ctx context.Context, in service.CreateOrderInput) (*model.Order, error) {
	return m.createFn(ctx, in)
}

func (m *mockOrderService) Get(ctx context.Context, orderID, userID uuid.UUID) (*model.Order, error) {
	return m.getFn(ctx, orderID, userID)
}

func (m *mockOrderService) List(ctx context.Context, in service.ListOrdersInput) ([]*model.Order, int32, error) {
	return m.listFn(ctx, in)
}

func (m *mockOrderService) Cancel(ctx context.Context, orderID, userID uuid.UUID, reason string) error {
	return m.cancelFn(ctx, orderID, userID, reason)
}

func sampleOrder() *model.Order {
	orderID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	userID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440001")
	productID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440002")
	sellerID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440003")
	return &model.Order{
		ID:          orderID,
		UserID:      userID,
		Status:      model.OrderStatusCreated,
		TotalAmount: 10000,
		Currency:    "JPY",
		Items: []model.OrderItem{
			{
				ID:          uuid.New(),
				OrderID:     orderID,
				ProductID:   productID,
				ProductName: "Keyboard",
				SellerID:    sellerID,
				UnitPrice:   5000,
				Quantity:    2,
				Subtotal:    10000,
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func TestOrderHandler_CreateOrder(t *testing.T) {
	order := sampleOrder()
	h := handler.NewOrderHandler(&mockOrderService{
		createFn: func(_ context.Context, in service.CreateOrderInput) (*model.Order, error) {
			assert.Equal(t, "idem-1", in.IdempotencyKey)
			assert.Len(t, in.Items, 1)
			assert.Equal(t, int32(2), in.Items[0].Quantity)
			return order, nil
		},
	})

	resp, err := h.CreateOrder(context.Background(), &orderv1.CreateOrderRequest{
		UserId:         order.UserID.String(),
		IdempotencyKey: "idem-1",
		Items: []*orderv1.OrderItemInput{
			{ProductId: order.Items[0].ProductID.String(), Quantity: 2},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, order.ID.String(), resp.Order.OrderId)
}

func TestOrderHandler_CreateOrder_InvalidUserID(t *testing.T) {
	h := handler.NewOrderHandler(&mockOrderService{})
	_, err := h.CreateOrder(context.Background(), &orderv1.CreateOrderRequest{UserId: "bad"})
	require.Error(t, err)
}

func TestOrderHandler_GetOrder(t *testing.T) {
	order := sampleOrder()
	h := handler.NewOrderHandler(&mockOrderService{
		getFn: func(_ context.Context, orderID, userID uuid.UUID) (*model.Order, error) {
			assert.Equal(t, order.ID, orderID)
			assert.Equal(t, order.UserID, userID)
			return order, nil
		},
	})

	resp, err := h.GetOrder(context.Background(), &orderv1.GetOrderRequest{
		OrderId: order.ID.String(),
		UserId:  order.UserID.String(),
	})
	require.NoError(t, err)
	assert.Equal(t, order.ID.String(), resp.Order.OrderId)
}

func TestOrderHandler_ListOrders(t *testing.T) {
	order := sampleOrder()
	h := handler.NewOrderHandler(&mockOrderService{
		listFn: func(_ context.Context, in service.ListOrdersInput) ([]*model.Order, int32, error) {
			assert.Equal(t, int32(2), in.Page)
			assert.Equal(t, int32(50), in.PageSize)
			return []*model.Order{order}, 1, nil
		},
	})

	resp, err := h.ListOrders(context.Background(), &orderv1.ListOrdersRequest{
		UserId:   order.UserID.String(),
		Page:     2,
		PageSize: 50,
	})
	require.NoError(t, err)
	assert.Len(t, resp.Orders, 1)
	assert.Equal(t, int32(1), resp.Total)
}

func TestOrderHandler_CancelOrder(t *testing.T) {
	called := false
	order := sampleOrder()
	h := handler.NewOrderHandler(&mockOrderService{
		cancelFn: func(_ context.Context, orderID, userID uuid.UUID, reason string) error {
			called = true
			assert.Equal(t, order.ID, orderID)
			assert.Equal(t, order.UserID, userID)
			assert.Equal(t, "changed_mind", reason)
			return nil
		},
	})

	_, err := h.CancelOrder(context.Background(), &orderv1.CancelOrderRequest{
		OrderId: order.ID.String(),
		UserId:  order.UserID.String(),
		Reason:  "changed_mind",
	})
	require.NoError(t, err)
	assert.True(t, called)
}

func TestOrderHandler_GetOrder_InvalidUUID(t *testing.T) {
	h := handler.NewOrderHandler(&mockOrderService{
		getFn: func(_ context.Context, _, _ uuid.UUID) (*model.Order, error) {
			return nil, apperrors.NewNotFound("order not found")
		},
	})

	_, err := h.GetOrder(context.Background(), &orderv1.GetOrderRequest{
		OrderId: "bad",
		UserId:  uuid.New().String(),
	})
	require.Error(t, err)
}
