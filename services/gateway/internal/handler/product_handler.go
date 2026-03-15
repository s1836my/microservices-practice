package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/yourorg/micromart/services/gateway/internal/client"
	productv1 "github.com/yourorg/micromart/proto/product/v1"
)

// ListProducts handles GET /api/v1/products
func (h *Handlers) ListProducts(c *gin.Context) {
	var query struct {
		CategoryID string `form:"category_id"`
		Page       int32  `form:"page,default=1"`
		PageSize   int32  `form:"page_size,default=20"`
	}
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: "INVALID_INPUT"})
		return
	}
	if query.PageSize > 100 {
		query.PageSize = 100
	}

	resp, err := client.Execute(h.clients.ProductCB, func() (*productv1.ListProductsResponse, error) {
		return h.clients.Product.ListProducts(c.Request.Context(), &productv1.ListProductsRequest{
			CategoryId: query.CategoryID,
			Page:       query.Page,
			PageSize:   query.PageSize,
		})
	})
	if err != nil {
		respondError(c, err)
		return
	}

	products := make([]*ProductResponse, 0, len(resp.Products))
	for _, p := range resp.Products {
		products = append(products, productFromProto(p))
	}

	c.JSON(http.StatusOK, gin.H{
		"data": products,
		"meta": PaginationMeta{
			Total:    resp.Total,
			Page:     query.Page,
			PageSize: query.PageSize,
		},
	})
}

// GetProduct handles GET /api/v1/products/:product_id
func (h *Handlers) GetProduct(c *gin.Context) {
	productID := c.Param("product_id")

	resp, err := client.Execute(h.clients.ProductCB, func() (*productv1.GetProductResponse, error) {
		return h.clients.Product.GetProduct(c.Request.Context(), &productv1.GetProductRequest{
			ProductId: productID,
		})
	})
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": productFromProto(resp.Product)})
}

// CreateProduct handles POST /api/v1/products (seller only)
func (h *Handlers) CreateProduct(c *gin.Context) {
	if contextUserRole(c) != "seller" && contextUserRole(c) != "admin" {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error: "only sellers can create products",
			Code:  "PERMISSION_DENIED",
		})
		return
	}

	var req struct {
		CategoryID   string   `json:"category_id"   binding:"required"`
		Name         string   `json:"name"          binding:"required,min=1,max=255"`
		Description  string   `json:"description"`
		Price        int64    `json:"price"         binding:"min=0"`
		InitialStock int32    `json:"initial_stock" binding:"min=0"`
		Images       []string `json:"images"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: "INVALID_INPUT"})
		return
	}

	sellerID := contextUserID(c)

	resp, err := client.Execute(h.clients.ProductCB, func() (*productv1.CreateProductResponse, error) {
		return h.clients.Product.CreateProduct(c.Request.Context(), &productv1.CreateProductRequest{
			SellerId:    sellerID,
			CategoryId:  req.CategoryID,
			Name:        req.Name,
			Description: req.Description,
			Price:       req.Price,
			InitialStock: req.InitialStock,
			Images:      req.Images,
		})
	})
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": productFromProto(resp.Product)})
}
