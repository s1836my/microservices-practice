package repository

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yourorg/micromart/services/cart/internal/model"
)

func TestCartKey(t *testing.T) {
	assert.Equal(t, "cart:user-1", cartKey("user-1"))
}

func TestCartField(t *testing.T) {
	assert.Equal(t, "product:prod-1", cartField("prod-1"))
}

func TestMarshalAndUnmarshalItem(t *testing.T) {
	item := &model.CartItem{
		ProductID:   "prod-1",
		ProductName: "Keyboard",
		UnitPrice:   5000,
		Quantity:    2,
	}

	encoded, err := marshalItem(item)
	require.NoError(t, err)

	decoded, err := unmarshalItem(encoded)
	require.NoError(t, err)
	assert.Equal(t, item.ProductID, decoded.ProductID)
	assert.Equal(t, item.ProductName, decoded.ProductName)
	assert.Equal(t, item.UnitPrice, decoded.UnitPrice)
	assert.Equal(t, item.Quantity, decoded.Quantity)
}

func TestCartRepository_CRUD(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
	})

	repo := NewCartRepository(client)
	ctx := context.Background()

	require.NoError(t, repo.Ping(ctx))
	require.NoError(t, repo.UpsertItem(ctx, "user-1", &model.CartItem{
		ProductID:   "prod-1",
		ProductName: "Keyboard",
		UnitPrice:   5000,
		Quantity:    2,
	}))
	require.True(t, mr.Exists("cart:user-1"))
	ttl := mr.TTL("cart:user-1")
	require.True(t, ttl > 0)

	items, err := repo.GetCart(ctx, "user-1")
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "Keyboard", items[0].ProductName)

	require.NoError(t, repo.RemoveItem(ctx, "user-1", "prod-1"))
	items, err = repo.GetCart(ctx, "user-1")
	require.NoError(t, err)
	assert.Empty(t, items)

	require.NoError(t, repo.UpsertItem(ctx, "user-1", &model.CartItem{
		ProductID: "prod-2",
		Quantity:  1,
	}))
	require.NoError(t, repo.ClearCart(ctx, "user-1"))
	assert.False(t, mr.Exists("cart:user-1"))
}
