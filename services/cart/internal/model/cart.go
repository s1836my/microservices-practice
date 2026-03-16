package model

import "sort"

type Cart struct {
	UserID     string
	Items      []*CartItem
	TotalPrice int64
	ItemCount  int32
}

type CartItem struct {
	ProductID   string `json:"product_id"`
	ProductName string `json:"product_name"`
	UnitPrice   int64  `json:"unit_price"`
	Quantity    int32  `json:"quantity"`
	Subtotal    int64  `json:"subtotal"`
}

func NewCart(userID string, items []*CartItem) *Cart {
	cloned := make([]*CartItem, 0, len(items))
	var totalPrice int64
	var itemCount int32

	for _, item := range items {
		if item == nil {
			continue
		}
		normalized := *item
		normalized.Subtotal = normalized.UnitPrice * int64(normalized.Quantity)
		totalPrice += normalized.Subtotal
		itemCount += normalized.Quantity
		cloned = append(cloned, &normalized)
	}

	sort.Slice(cloned, func(i, j int) bool {
		return cloned[i].ProductID < cloned[j].ProductID
	})

	return &Cart{
		UserID:     userID,
		Items:      cloned,
		TotalPrice: totalPrice,
		ItemCount:  itemCount,
	}
}
