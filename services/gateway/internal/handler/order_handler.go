package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/yourorg/micromart/services/gateway/internal/client"
	orderv1 "github.com/yourorg/micromart/proto/order/v1"
)

// CreateOrder handles POST /api/v1/orders
func (h *Handlers) CreateOrder(c *gin.Context) {
	userID := contextUserID(c)

	var req struct {
		IdempotencyKey string `json:"idempotency_key" binding:"required"`
		Items          []struct {
			ProductID string `json:"product_id" binding:"required"`
			Quantity  int32  `json:"quantity"   binding:"required,min=1"`
		} `json:"items" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: "INVALID_INPUT"})
		return
	}

	orderItems := make([]*orderv1.OrderItemInput, 0, len(req.Items))
	for _, item := range req.Items {
		orderItems = append(orderItems, &orderv1.OrderItemInput{
			ProductId: item.ProductID,
			Quantity:  item.Quantity,
		})
	}

	resp, err := client.Execute(h.clients.OrderCB, func() (*orderv1.CreateOrderResponse, error) {
		return h.clients.Order.CreateOrder(c.Request.Context(), &orderv1.CreateOrderRequest{
			UserId:         userID,
			IdempotencyKey: req.IdempotencyKey,
			Items:          orderItems,
		})
	})
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"data": orderFromProto(resp.Order)})
}

// ListOrders handles GET /api/v1/orders
func (h *Handlers) ListOrders(c *gin.Context) {
	userID := contextUserID(c)

	var query struct {
		Page     int32 `form:"page,default=1"`
		PageSize int32 `form:"page_size,default=20"`
	}
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: "INVALID_INPUT"})
		return
	}

	resp, err := client.Execute(h.clients.OrderCB, func() (*orderv1.ListOrdersResponse, error) {
		return h.clients.Order.ListOrders(c.Request.Context(), &orderv1.ListOrdersRequest{
			UserId:   userID,
			Page:     query.Page,
			PageSize: query.PageSize,
		})
	})
	if err != nil {
		respondError(c, err)
		return
	}

	orders := make([]*OrderResponse, 0, len(resp.Orders))
	for _, o := range resp.Orders {
		orders = append(orders, orderFromProto(o))
	}

	c.JSON(http.StatusOK, gin.H{
		"data": orders,
		"meta": PaginationMeta{
			Total:    resp.Total,
			Page:     query.Page,
			PageSize: query.PageSize,
		},
	})
}

// GetOrder handles GET /api/v1/orders/:order_id
func (h *Handlers) GetOrder(c *gin.Context) {
	userID := contextUserID(c)
	orderID := c.Param("order_id")

	resp, err := client.Execute(h.clients.OrderCB, func() (*orderv1.GetOrderResponse, error) {
		return h.clients.Order.GetOrder(c.Request.Context(), &orderv1.GetOrderRequest{
			OrderId: orderID,
			UserId:  userID,
		})
	})
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": orderFromProto(resp.Order)})
}
