package repository

import (
	"context"

	"github.com/yourorg/micromart/services/search/internal/model"
)

// SearchFilter carries search criteria and pagination.
type SearchFilter struct {
	Query      string
	CategoryID string
	PriceMin   int64
	PriceMax   int64
	SortBy     string
	Page       int32
	PageSize   int32
}

// SearchRepository persists and queries product search documents.
type SearchRepository interface {
	Ping(ctx context.Context) error
	EnsureIndex(ctx context.Context) error
	Search(ctx context.Context, filter SearchFilter) ([]*model.ProductDocument, int64, error)
	UpsertProduct(ctx context.Context, product *model.ProductDocument) error
	DeleteProduct(ctx context.Context, productID string) error
}
