package handler_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apperrors "github.com/yourorg/micromart/pkg/errors"
	searchv1 "github.com/yourorg/micromart/proto/search/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/yourorg/micromart/services/search/internal/handler"
	"github.com/yourorg/micromart/services/search/internal/model"
)

type mockSearchService struct {
	searchFn func(ctx context.Context, req *searchv1.SearchProductsRequest) ([]*model.ProductDocument, int64, int32, int32, error)
}

func (m *mockSearchService) Search(ctx context.Context, req *searchv1.SearchProductsRequest) ([]*model.ProductDocument, int64, int32, int32, error) {
	return m.searchFn(ctx, req)
}

func TestSearchHandler_SearchProducts(t *testing.T) {
	svc := &mockSearchService{
		searchFn: func(_ context.Context, _ *searchv1.SearchProductsRequest) ([]*model.ProductDocument, int64, int32, int32, error) {
			return []*model.ProductDocument{{
				ProductID:   "prod-1",
				Name:        "Laptop",
				Description: "14-inch",
				Price:       150000,
				CategoryID:  "cat-1",
				SellerID:    "seller-1",
				Images:      []string{"img-1"},
				Score:       1.23,
			}}, 1, 2, 10, nil
		},
	}
	h := handler.NewSearchHandler(svc)

	resp, err := h.SearchProducts(context.Background(), &searchv1.SearchProductsRequest{Query: "laptop"})
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "prod-1", resp.Items[0].ProductId)
	assert.Equal(t, int64(1), resp.Total)
	assert.Equal(t, int32(2), resp.Page)
}

func TestSearchHandler_SearchProducts_MapsGRPCErrors(t *testing.T) {
	svc := &mockSearchService{
		searchFn: func(_ context.Context, _ *searchv1.SearchProductsRequest) ([]*model.ProductDocument, int64, int32, int32, error) {
			return nil, 0, 0, 0, apperrors.NewInvalidInput("bad query")
		},
	}
	h := handler.NewSearchHandler(svc)

	_, err := h.SearchProducts(context.Background(), &searchv1.SearchProductsRequest{Query: "x"})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Equal(t, "bad query", st.Message())
}
