package service_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apperrors "github.com/yourorg/micromart/pkg/errors"
	"github.com/yourorg/micromart/services/cart/internal/model"
	"github.com/yourorg/micromart/services/cart/internal/service"
)

type mockCartRepository struct {
	getCartFn    func(ctx context.Context, userID string) ([]*model.CartItem, error)
	upsertItemFn func(ctx context.Context, userID string, item *model.CartItem) error
	removeItemFn func(ctx context.Context, userID, productID string) error
	clearCartFn  func(ctx context.Context, userID string) error
}

func (m *mockCartRepository) Ping(_ context.Context) error { return nil }

func (m *mockCartRepository) GetCart(ctx context.Context, userID string) ([]*model.CartItem, error) {
	return m.getCartFn(ctx, userID)
}

func (m *mockCartRepository) UpsertItem(ctx context.Context, userID string, item *model.CartItem) error {
	return m.upsertItemFn(ctx, userID, item)
}

func (m *mockCartRepository) RemoveItem(ctx context.Context, userID, productID string) error {
	return m.removeItemFn(ctx, userID, productID)
}

func (m *mockCartRepository) ClearCart(ctx context.Context, userID string) error {
	return m.clearCartFn(ctx, userID)
}

func TestCartService_GetCart_ComputesTotals(t *testing.T) {
	repo := &mockCartRepository{
		getCartFn: func(_ context.Context, _ string) ([]*model.CartItem, error) {
			return []*model.CartItem{
				{ProductID: "prod-2", ProductName: "Mouse", UnitPrice: 2000, Quantity: 1},
				{ProductID: "prod-1", ProductName: "Keyboard", UnitPrice: 5000, Quantity: 2},
			}, nil
		},
	}
	svc := service.NewCartService(repo)

	cart, err := svc.GetCart(context.Background(), "user-1")
	require.NoError(t, err)
	require.Len(t, cart.Items, 2)
	assert.Equal(t, "prod-1", cart.Items[0].ProductID)
	assert.Equal(t, int64(12000), cart.TotalPrice)
	assert.Equal(t, int32(3), cart.ItemCount)
	assert.Equal(t, int64(10000), cart.Items[0].Subtotal)
}

func TestCartService_AddItem_IncrementsExistingQuantity(t *testing.T) {
	var saved *model.CartItem
	repo := &mockCartRepository{
		getCartFn: func(_ context.Context, _ string) ([]*model.CartItem, error) {
			return []*model.CartItem{
				{ProductID: "prod-1", ProductName: "Keyboard", UnitPrice: 5000, Quantity: 2},
			}, nil
		},
		upsertItemFn: func(_ context.Context, _ string, item *model.CartItem) error {
			saved = item
			return nil
		},
	}
	svc := service.NewCartService(repo)

	cart, err := svc.AddItem(context.Background(), "user-1", "prod-1", 3)
	require.NoError(t, err)
	require.NotNil(t, saved)
	assert.Equal(t, int32(5), saved.Quantity)
	assert.Equal(t, int32(5), cart.ItemCount)
	assert.Equal(t, int64(25000), cart.TotalPrice)
}

func TestCartService_AddItem_CreatesNewItem(t *testing.T) {
	var saved *model.CartItem
	repo := &mockCartRepository{
		getCartFn: func(_ context.Context, _ string) ([]*model.CartItem, error) {
			return []*model.CartItem{}, nil
		},
		upsertItemFn: func(_ context.Context, _ string, item *model.CartItem) error {
			saved = item
			return nil
		},
	}
	svc := service.NewCartService(repo)

	cart, err := svc.AddItem(context.Background(), "user-1", "prod-9", 1)
	require.NoError(t, err)
	require.NotNil(t, saved)
	assert.Equal(t, "prod-9", saved.ProductID)
	assert.Equal(t, int32(1), saved.Quantity)
	assert.Len(t, cart.Items, 1)
}

func TestCartService_UpdateItem_RemovesWhenQuantityZero(t *testing.T) {
	removed := false
	repo := &mockCartRepository{
		getCartFn: func(_ context.Context, _ string) ([]*model.CartItem, error) {
			return []*model.CartItem{
				{ProductID: "prod-1", Quantity: 2},
			}, nil
		},
		removeItemFn: func(_ context.Context, _ string, productID string) error {
			removed = true
			assert.Equal(t, "prod-1", productID)
			return nil
		},
	}
	svc := service.NewCartService(repo)

	cart, err := svc.UpdateItem(context.Background(), "user-1", "prod-1", 0)
	require.NoError(t, err)
	assert.True(t, removed)
	assert.Empty(t, cart.Items)
}

func TestCartService_UpdateItem_RejectsMissingItem(t *testing.T) {
	repo := &mockCartRepository{
		getCartFn: func(_ context.Context, _ string) ([]*model.CartItem, error) {
			return []*model.CartItem{
				{ProductID: "prod-2", Quantity: 1},
			}, nil
		},
		upsertItemFn: func(_ context.Context, _ string, _ *model.CartItem) error {
			t.Fatal("unexpected upsert")
			return nil
		},
	}
	svc := service.NewCartService(repo)

	_, err := svc.UpdateItem(context.Background(), "user-1", "prod-1", 2)
	require.Error(t, err)
	appErr, ok := apperrors.AsAppError(err)
	require.True(t, ok)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

func TestCartService_UpdateItem_UpdatesExistingItem(t *testing.T) {
	var saved *model.CartItem
	repo := &mockCartRepository{
		getCartFn: func(_ context.Context, _ string) ([]*model.CartItem, error) {
			return []*model.CartItem{
				{ProductID: "prod-1", ProductName: "Keyboard", UnitPrice: 5000, Quantity: 1},
			}, nil
		},
		upsertItemFn: func(_ context.Context, _ string, item *model.CartItem) error {
			saved = item
			return nil
		},
	}
	svc := service.NewCartService(repo)

	cart, err := svc.UpdateItem(context.Background(), "user-1", "prod-1", 4)
	require.NoError(t, err)
	require.NotNil(t, saved)
	assert.Equal(t, int32(4), saved.Quantity)
	assert.Equal(t, int32(4), cart.ItemCount)
	assert.Equal(t, int64(20000), cart.TotalPrice)
}

func TestCartService_AddItem_RejectsInvalidQuantity(t *testing.T) {
	svc := service.NewCartService(&mockCartRepository{
		getCartFn: func(_ context.Context, _ string) ([]*model.CartItem, error) {
			t.Fatal("unexpected get cart")
			return nil, nil
		},
	})

	_, err := svc.AddItem(context.Background(), "user-1", "prod-1", 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "quantity")
}

func TestCartService_RemoveItem_ValidatesInput(t *testing.T) {
	svc := service.NewCartService(&mockCartRepository{
		removeItemFn: func(_ context.Context, _, _ string) error {
			t.Fatal("unexpected remove")
			return nil
		},
	})

	err := svc.RemoveItem(context.Background(), "", "prod-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "user_id")
}

func TestCartService_GetCart_ValidatesInput(t *testing.T) {
	svc := service.NewCartService(&mockCartRepository{
		getCartFn: func(_ context.Context, _ string) ([]*model.CartItem, error) {
			t.Fatal("unexpected get cart")
			return nil, nil
		},
	})

	_, err := svc.GetCart(context.Background(), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "user_id")
}

func TestCartService_ClearCart_Delegates(t *testing.T) {
	cleared := false
	svc := service.NewCartService(&mockCartRepository{
		clearCartFn: func(_ context.Context, userID string) error {
			cleared = true
			assert.Equal(t, "user-1", userID)
			return nil
		},
	})

	err := svc.ClearCart(context.Background(), "user-1")
	require.NoError(t, err)
	assert.True(t, cleared)
}
