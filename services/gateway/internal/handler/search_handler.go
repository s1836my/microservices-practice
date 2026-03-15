package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/yourorg/micromart/services/gateway/internal/client"
	searchv1 "github.com/yourorg/micromart/proto/search/v1"
)

// SearchProducts handles GET /api/v1/products/search
func (h *Handlers) SearchProducts(c *gin.Context) {
	var query struct {
		Q          string  `form:"q"          binding:"required,min=1"`
		CategoryID string  `form:"category_id"`
		PriceMin   int64   `form:"price_min"`
		PriceMax   int64   `form:"price_max"`
		SortBy     string  `form:"sort_by,default=relevance"`
		Page       int32   `form:"page,default=1"`
		PageSize   int32   `form:"page_size,default=20"`
	}
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: "INVALID_INPUT"})
		return
	}
	if query.PageSize > 50 {
		query.PageSize = 50
	}

	resp, err := client.Execute(h.clients.SearchCB, func() (*searchv1.SearchProductsResponse, error) {
		return h.clients.Search.SearchProducts(c.Request.Context(), &searchv1.SearchProductsRequest{
			Query:      query.Q,
			CategoryId: query.CategoryID,
			PriceMin:   query.PriceMin,
			PriceMax:   query.PriceMax,
			SortBy:     query.SortBy,
			Page:       query.Page,
			PageSize:   query.PageSize,
		})
	})
	if err != nil {
		respondError(c, err)
		return
	}

	products := make([]*ProductResponse, 0, len(resp.Items))
	for _, item := range resp.Items {
		products = append(products, productFromSearchItem(item))
	}

	c.JSON(http.StatusOK, gin.H{
		"data": products,
		"meta": PaginationMeta{
			Total:    int32(resp.Total),
			Page:     resp.Page,
			PageSize: resp.PageSize,
		},
	})
}
