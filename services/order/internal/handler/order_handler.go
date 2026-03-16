package handler

import (
	"context"

	"github.com/google/uuid"
	apperrors "github.com/yourorg/micromart/pkg/errors"
	orderv1 "github.com/yourorg/micromart/proto/order/v1"
	"github.com/yourorg/micromart/services/order/internal/service"
)

type OrderHandler struct {
	orderv1.UnimplementedOrderServiceServer
	orderService service.OrderService
}

func NewOrderHandler(orderService service.OrderService) *OrderHandler {
	return &OrderHandler{orderService: orderService}
}

func (h *OrderHandler) CreateOrder(ctx context.Context, req *orderv1.CreateOrderRequest) (*orderv1.CreateOrderResponse, error) {
	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, apperrors.NewInvalidInput("invalid user_id")
	}

	items := make([]service.CreateItemInput, 0, len(req.Items))
	for idx, item := range req.Items {
		productID, err := uuid.Parse(item.ProductId)
		if err != nil {
			return nil, apperrors.NewInvalidInput("invalid items[%d].product_id", idx)
		}
		items = append(items, service.CreateItemInput{
			ProductID: productID,
			Quantity:  item.Quantity,
		})
	}

	order, err := h.orderService.Create(ctx, service.CreateOrderInput{
		UserID:         userID,
		IdempotencyKey: req.IdempotencyKey,
		Items:          items,
	})
	if err != nil {
		return nil, err
	}
	return &orderv1.CreateOrderResponse{Order: order.ToProto()}, nil
}

func (h *OrderHandler) GetOrder(ctx context.Context, req *orderv1.GetOrderRequest) (*orderv1.GetOrderResponse, error) {
	orderID, err := uuid.Parse(req.OrderId)
	if err != nil {
		return nil, apperrors.NewInvalidInput("invalid order_id")
	}
	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, apperrors.NewInvalidInput("invalid user_id")
	}

	order, err := h.orderService.Get(ctx, orderID, userID)
	if err != nil {
		return nil, err
	}
	return &orderv1.GetOrderResponse{Order: order.ToProto()}, nil
}

func (h *OrderHandler) ListOrders(ctx context.Context, req *orderv1.ListOrdersRequest) (*orderv1.ListOrdersResponse, error) {
	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, apperrors.NewInvalidInput("invalid user_id")
	}

	orders, total, err := h.orderService.List(ctx, service.ListOrdersInput{
		UserID:   userID,
		Page:     req.Page,
		PageSize: req.PageSize,
	})
	if err != nil {
		return nil, err
	}

	respOrders := make([]*orderv1.Order, 0, len(orders))
	for _, order := range orders {
		respOrders = append(respOrders, order.ToProto())
	}
	return &orderv1.ListOrdersResponse{Orders: respOrders, Total: total}, nil
}

func (h *OrderHandler) CancelOrder(ctx context.Context, req *orderv1.CancelOrderRequest) (*orderv1.CancelOrderResponse, error) {
	orderID, err := uuid.Parse(req.OrderId)
	if err != nil {
		return nil, apperrors.NewInvalidInput("invalid order_id")
	}
	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, apperrors.NewInvalidInput("invalid user_id")
	}

	if err := h.orderService.Cancel(ctx, orderID, userID, req.Reason); err != nil {
		return nil, err
	}
	return &orderv1.CancelOrderResponse{}, nil
}
