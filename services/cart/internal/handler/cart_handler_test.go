package handler_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	cartv1 "github.com/yourorg/micromart/proto/cart/v1"
	"github.com/yourorg/micromart/services/cart/internal/handler"
	"github.com/yourorg/micromart/services/cart/internal/model"
)

type mockCartService struct {
	getCartFn    func(ctx context.Context, userID string) (*model.Cart, error)
	addItemFn    func(ctx context.Context, userID, productID string, quantity int32) (*model.Cart, error)
	updateItemFn func(ctx context.Context, userID, productID string, quantity int32) (*model.Cart, error)
	removeItemFn func(ctx context.Context, userID, productID string) error
	clearCartFn  func(ctx context.Context, userID string) error
}

func (m *mockCartService) GetCart(ctx context.Context, userID string) (*model.Cart, error) {
	return m.getCartFn(ctx, userID)
}

func (m *mockCartService) AddItem(ctx context.Context, userID, productID string, quantity int32) (*model.Cart, error) {
	return m.addItemFn(ctx, userID, productID, quantity)
}

func (m *mockCartService) UpdateItem(ctx context.Context, userID, productID string, quantity int32) (*model.Cart, error) {
	return m.updateItemFn(ctx, userID, productID, quantity)
}

func (m *mockCartService) RemoveItem(ctx context.Context, userID, productID string) error {
	return m.removeItemFn(ctx, userID, productID)
}

func (m *mockCartService) ClearCart(ctx context.Context, userID string) error {
	return m.clearCartFn(ctx, userID)
}

func sampleCart() *model.Cart {
	return &model.Cart{
		UserID: "user-1",
		Items: []*model.CartItem{{
			ProductID:   "prod-1",
			ProductName: "Keyboard",
			UnitPrice:   5000,
			Quantity:    2,
			Subtotal:    10000,
		}},
		TotalPrice: 10000,
		ItemCount:  2,
	}
}

func TestCartHandler_GetCart(t *testing.T) {
	h := handler.NewCartHandler(&mockCartService{
		getCartFn: func(_ context.Context, userID string) (*model.Cart, error) {
			assert.Equal(t, "user-1", userID)
			return sampleCart(), nil
		},
	})

	resp, err := h.GetCart(context.Background(), &cartv1.GetCartRequest{UserId: "user-1"})
	require.NoError(t, err)
	require.NotNil(t, resp.Cart)
	assert.Equal(t, int32(2), resp.Cart.ItemCount)
	assert.Equal(t, "prod-1", resp.Cart.Items[0].ProductId)
}

func TestCartHandler_AddItem(t *testing.T) {
	h := handler.NewCartHandler(&mockCartService{
		addItemFn: func(_ context.Context, userID, productID string, quantity int32) (*model.Cart, error) {
			assert.Equal(t, "user-1", userID)
			assert.Equal(t, "prod-1", productID)
			assert.Equal(t, int32(2), quantity)
			return sampleCart(), nil
		},
	})

	resp, err := h.AddItem(context.Background(), &cartv1.AddItemRequest{
		UserId:    "user-1",
		ProductId: "prod-1",
		Quantity:  2,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(10000), resp.Cart.TotalPrice)
}

func TestCartHandler_UpdateItem(t *testing.T) {
	h := handler.NewCartHandler(&mockCartService{
		updateItemFn: func(_ context.Context, userID, productID string, quantity int32) (*model.Cart, error) {
			assert.Equal(t, "user-1", userID)
			assert.Equal(t, "prod-1", productID)
			assert.Equal(t, int32(5), quantity)
			return sampleCart(), nil
		},
	})

	resp, err := h.UpdateItem(context.Background(), &cartv1.UpdateItemRequest{
		UserId:    "user-1",
		ProductId: "prod-1",
		Quantity:  5,
	})
	require.NoError(t, err)
	assert.Equal(t, "Keyboard", resp.Cart.Items[0].ProductName)
}

func TestCartHandler_RemoveItem(t *testing.T) {
	called := false
	h := handler.NewCartHandler(&mockCartService{
		removeItemFn: func(_ context.Context, userID, productID string) error {
			called = true
			assert.Equal(t, "user-1", userID)
			assert.Equal(t, "prod-1", productID)
			return nil
		},
	})

	_, err := h.RemoveItem(context.Background(), &cartv1.RemoveItemRequest{
		UserId:    "user-1",
		ProductId: "prod-1",
	})
	require.NoError(t, err)
	assert.True(t, called)
}

func TestCartHandler_ClearCart(t *testing.T) {
	called := false
	h := handler.NewCartHandler(&mockCartService{
		clearCartFn: func(_ context.Context, userID string) error {
			called = true
			assert.Equal(t, "user-1", userID)
			return nil
		},
	})

	_, err := h.ClearCart(context.Background(), &cartv1.ClearCartRequest{UserId: "user-1"})
	require.NoError(t, err)
	assert.True(t, called)
}

func TestCartHandler_GetCart_ReturnsServiceError(t *testing.T) {
	expected := assert.AnError
	h := handler.NewCartHandler(&mockCartService{
		getCartFn: func(_ context.Context, _ string) (*model.Cart, error) {
			return nil, expected
		},
	})

	_, err := h.GetCart(context.Background(), &cartv1.GetCartRequest{UserId: "user-1"})
	require.ErrorIs(t, err, expected)
}
