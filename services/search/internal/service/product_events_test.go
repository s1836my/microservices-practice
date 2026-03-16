package service_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yourorg/micromart/services/search/internal/model"
	"github.com/yourorg/micromart/services/search/internal/service"
)

type mockIndexer struct {
	upserted []*model.ProductDocument
	deleted  []string
}

func (m *mockIndexer) UpsertProduct(_ context.Context, product *model.ProductDocument) error {
	m.upserted = append(m.upserted, product)
	return nil
}

func (m *mockIndexer) DeleteProduct(_ context.Context, productID string) error {
	m.deleted = append(m.deleted, productID)
	return nil
}

func TestProductEventProcessor_HandleMessage_UpsertsCreatedProducts(t *testing.T) {
	indexer := &mockIndexer{}
	processor := service.NewProductEventProcessor(indexer, slog.Default())

	err := processor.HandleMessage(context.Background(), []byte(`{
		"event_type":"product.created",
		"payload":{
			"product_id":"prod-1",
			"name":"Keyboard",
			"description":"Mechanical",
			"price":12000,
			"category_id":"cat-1",
			"seller_id":"seller-1",
			"images":["img-1"],
			"status":"active",
			"stock":10
		}
	}`))
	require.NoError(t, err)
	require.Len(t, indexer.upserted, 1)
	assert.Equal(t, "prod-1", indexer.upserted[0].ProductID)
}

func TestProductEventProcessor_HandleMessage_DeletesRemovedProducts(t *testing.T) {
	indexer := &mockIndexer{}
	processor := service.NewProductEventProcessor(indexer, slog.Default())

	err := processor.HandleMessage(context.Background(), []byte(`{
		"event_type":"product.deleted",
		"payload":{"product_id":"prod-9"}
	}`))
	require.NoError(t, err)
	assert.Equal(t, []string{"prod-9"}, indexer.deleted)
}
