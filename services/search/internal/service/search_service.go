package service

import (
	"context"
	"strings"

	apperrors "github.com/yourorg/micromart/pkg/errors"
	searchv1 "github.com/yourorg/micromart/proto/search/v1"
	"github.com/yourorg/micromart/services/search/internal/model"
	"github.com/yourorg/micromart/services/search/internal/repository"
)

// SearchService exposes search use cases.
type SearchService interface {
	Search(ctx context.Context, req *searchv1.SearchProductsRequest) ([]*model.ProductDocument, int64, int32, int32, error)
}

type searchService struct {
	repo repository.SearchRepository
}

// NewSearchService creates a SearchService.
func NewSearchService(repo repository.SearchRepository) SearchService {
	return &searchService{repo: repo}
}

func (s *searchService) Search(ctx context.Context, req *searchv1.SearchProductsRequest) ([]*model.ProductDocument, int64, int32, int32, error) {
	query := strings.TrimSpace(req.GetQuery())
	if query == "" {
		return nil, 0, 0, 0, apperrors.NewInvalidInput("query is required")
	}
	if req.GetPriceMin() > 0 && req.GetPriceMax() > 0 && req.GetPriceMin() > req.GetPriceMax() {
		return nil, 0, 0, 0, apperrors.NewInvalidInput("price_min must be less than or equal to price_max")
	}

	page := req.GetPage()
	if page <= 0 {
		page = 1
	}
	pageSize := req.GetPageSize()
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 50 {
		pageSize = 50
	}

	sortBy := req.GetSortBy()
	switch sortBy {
	case "", "relevance", "price_asc", "price_desc", "newest":
		if sortBy == "" {
			sortBy = "relevance"
		}
	default:
		return nil, 0, 0, 0, apperrors.NewInvalidInput("unsupported sort_by")
	}

	items, total, err := s.repo.Search(ctx, repository.SearchFilter{
		Query:      query,
		CategoryID: req.GetCategoryId(),
		PriceMin:   req.GetPriceMin(),
		PriceMax:   req.GetPriceMax(),
		SortBy:     sortBy,
		Page:       page,
		PageSize:   pageSize,
	})
	if err != nil {
		return nil, 0, 0, 0, err
	}
	return items, total, page, pageSize, nil
}
