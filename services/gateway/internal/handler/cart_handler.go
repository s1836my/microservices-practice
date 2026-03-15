package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/yourorg/micromart/services/gateway/internal/client"
	cartv1 "github.com/yourorg/micromart/proto/cart/v1"
)

// GetCart handles GET /api/v1/cart
func (h *Handlers) GetCart(c *gin.Context) {
	userID := contextUserID(c)

	resp, err := client.Execute(h.clients.CartCB, func() (*cartv1.GetCartResponse, error) {
		return h.clients.Cart.GetCart(c.Request.Context(), &cartv1.GetCartRequest{
			UserId: userID,
		})
	})
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": cartFromProto(resp.Cart)})
}

// AddCartItem handles POST /api/v1/cart/items
func (h *Handlers) AddCartItem(c *gin.Context) {
	userID := contextUserID(c)

	var req struct {
		ProductID string `json:"product_id" binding:"required"`
		Quantity  int32  `json:"quantity"   binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: "INVALID_INPUT"})
		return
	}

	resp, err := client.Execute(h.clients.CartCB, func() (*cartv1.AddItemResponse, error) {
		return h.clients.Cart.AddItem(c.Request.Context(), &cartv1.AddItemRequest{
			UserId:    userID,
			ProductId: req.ProductID,
			Quantity:  req.Quantity,
		})
	})
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": cartFromProto(resp.Cart)})
}

// RemoveCartItem handles DELETE /api/v1/cart/items/:product_id
func (h *Handlers) RemoveCartItem(c *gin.Context) {
	userID := contextUserID(c)
	productID := c.Param("product_id")

	_, err := client.Execute(h.clients.CartCB, func() (*cartv1.RemoveItemResponse, error) {
		return h.clients.Cart.RemoveItem(c.Request.Context(), &cartv1.RemoveItemRequest{
			UserId:    userID,
			ProductId: productID,
		})
	})
	if err != nil {
		respondError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}
