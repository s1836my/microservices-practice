package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yourorg/micromart/services/cart/internal/model"
)

func TestNewCart_ComputesTotals(t *testing.T) {
	cart := model.NewCart("user-1", []*model.CartItem{
		{ProductID: "prod-2", UnitPrice: 2000, Quantity: 1},
		{ProductID: "prod-1", UnitPrice: 5000, Quantity: 3},
	})

	assert.Equal(t, "user-1", cart.UserID)
	assert.Equal(t, int64(17000), cart.TotalPrice)
	assert.Equal(t, int32(4), cart.ItemCount)
	assert.Equal(t, "prod-1", cart.Items[0].ProductID)
	assert.Equal(t, int64(15000), cart.Items[0].Subtotal)
}
