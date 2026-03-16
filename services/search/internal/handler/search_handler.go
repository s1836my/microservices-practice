package handler

import (
	"context"

	apperrors "github.com/yourorg/micromart/pkg/errors"
	searchv1 "github.com/yourorg/micromart/proto/search/v1"
	"github.com/yourorg/micromart/services/search/internal/service"
)

// SearchHandler implements the SearchService gRPC server.
type SearchHandler struct {
	searchv1.UnimplementedSearchServiceServer
	svc service.SearchService
}

// NewSearchHandler creates a SearchHandler.
func NewSearchHandler(svc service.SearchService) *SearchHandler {
	return &SearchHandler{svc: svc}
}

func (h *SearchHandler) SearchProducts(ctx context.Context, req *searchv1.SearchProductsRequest) (*searchv1.SearchProductsResponse, error) {
	items, total, page, pageSize, err := h.svc.Search(ctx, req)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err).Err()
	}

	respItems := make([]*searchv1.SearchResultItem, 0, len(items))
	for _, item := range items {
		respItems = append(respItems, &searchv1.SearchResultItem{
			ProductId:   item.ProductID,
			Name:        item.Name,
			Description: item.Description,
			Price:       item.Price,
			CategoryId:  item.CategoryID,
			SellerId:    item.SellerID,
			Images:      item.Images,
			Score:       item.Score,
		})
	}

	return &searchv1.SearchProductsResponse{
		Items:    respItems,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}
