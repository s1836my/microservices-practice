package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yourorg/micromart/services/cart/internal/model"
)

func TestCartToProto_Nil(t *testing.T) {
	assert.Nil(t, cartToProto(nil))
}

func TestCartToProto_SkipsNilItems(t *testing.T) {
	cart := cartToProto(&model.Cart{
		UserID: "user-1",
		Items: []*model.CartItem{
			nil,
			{ProductID: "prod-1", Quantity: 2, Subtotal: 100},
		},
		ItemCount:  2,
		TotalPrice: 100,
	})

	assert.Len(t, cart.Items, 1)
	assert.Equal(t, "prod-1", cart.Items[0].ProductId)
}
