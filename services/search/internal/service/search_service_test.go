package service_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	searchv1 "github.com/yourorg/micromart/proto/search/v1"
	"github.com/yourorg/micromart/services/search/internal/model"
	"github.com/yourorg/micromart/services/search/internal/repository"
	"github.com/yourorg/micromart/services/search/internal/service"
)

type mockSearchRepo struct {
	searchFn func(ctx context.Context, filter repository.SearchFilter) ([]*model.ProductDocument, int64, error)
}

func (m *mockSearchRepo) Ping(_ context.Context) error        { return nil }
func (m *mockSearchRepo) EnsureIndex(_ context.Context) error { return nil }
func (m *mockSearchRepo) Search(ctx context.Context, filter repository.SearchFilter) ([]*model.ProductDocument, int64, error) {
	return m.searchFn(ctx, filter)
}
func (m *mockSearchRepo) UpsertProduct(_ context.Context, _ *model.ProductDocument) error { return nil }
func (m *mockSearchRepo) DeleteProduct(_ context.Context, _ string) error                 { return nil }

func TestSearchService_Search_NormalizesRequest(t *testing.T) {
	var got repository.SearchFilter
	repo := &mockSearchRepo{
		searchFn: func(_ context.Context, filter repository.SearchFilter) ([]*model.ProductDocument, int64, error) {
			got = filter
			return []*model.ProductDocument{{ProductID: "prod-1", Name: "Laptop"}}, 1, nil
		},
	}
	svc := service.NewSearchService(repo)

	items, total, page, pageSize, err := svc.Search(context.Background(), &searchv1.SearchProductsRequest{
		Query:    "  laptop  ",
		Page:     0,
		PageSize: 999,
	})
	require.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, int32(1), page)
	assert.Equal(t, int32(50), pageSize)
	assert.Equal(t, "laptop", got.Query)
	assert.Equal(t, "relevance", got.SortBy)
	assert.Equal(t, int32(1), got.Page)
	assert.Equal(t, int32(50), got.PageSize)
}

func TestSearchService_Search_RejectsEmptyQuery(t *testing.T) {
	svc := service.NewSearchService(&mockSearchRepo{})

	_, _, _, _, err := svc.Search(context.Background(), &searchv1.SearchProductsRequest{Query: "   "})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "query is required")
}

func TestSearchService_Search_RejectsInvalidPriceRange(t *testing.T) {
	svc := service.NewSearchService(&mockSearchRepo{})

	_, _, _, _, err := svc.Search(context.Background(), &searchv1.SearchProductsRequest{
		Query:    "phone",
		PriceMin: 200,
		PriceMax: 100,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "price_min")
}
