package handler

import (
	"context"

	cartv1 "github.com/yourorg/micromart/proto/cart/v1"
	"github.com/yourorg/micromart/services/cart/internal/model"
	"github.com/yourorg/micromart/services/cart/internal/service"
)

type CartHandler struct {
	cartv1.UnimplementedCartServiceServer
	svc service.CartService
}

func NewCartHandler(svc service.CartService) *CartHandler {
	return &CartHandler{svc: svc}
}

func (h *CartHandler) GetCart(ctx context.Context, req *cartv1.GetCartRequest) (*cartv1.GetCartResponse, error) {
	cart, err := h.svc.GetCart(ctx, req.UserId)
	if err != nil {
		return nil, err
	}
	return &cartv1.GetCartResponse{Cart: cartToProto(cart)}, nil
}

func (h *CartHandler) AddItem(ctx context.Context, req *cartv1.AddItemRequest) (*cartv1.AddItemResponse, error) {
	cart, err := h.svc.AddItem(ctx, req.UserId, req.ProductId, req.Quantity)
	if err != nil {
		return nil, err
	}
	return &cartv1.AddItemResponse{Cart: cartToProto(cart)}, nil
}

func (h *CartHandler) UpdateItem(ctx context.Context, req *cartv1.UpdateItemRequest) (*cartv1.UpdateItemResponse, error) {
	cart, err := h.svc.UpdateItem(ctx, req.UserId, req.ProductId, req.Quantity)
	if err != nil {
		return nil, err
	}
	return &cartv1.UpdateItemResponse{Cart: cartToProto(cart)}, nil
}

func (h *CartHandler) RemoveItem(ctx context.Context, req *cartv1.RemoveItemRequest) (*cartv1.RemoveItemResponse, error) {
	if err := h.svc.RemoveItem(ctx, req.UserId, req.ProductId); err != nil {
		return nil, err
	}
	return &cartv1.RemoveItemResponse{}, nil
}

func (h *CartHandler) ClearCart(ctx context.Context, req *cartv1.ClearCartRequest) (*cartv1.ClearCartResponse, error) {
	if err := h.svc.ClearCart(ctx, req.UserId); err != nil {
		return nil, err
	}
	return &cartv1.ClearCartResponse{}, nil
}

func cartToProto(cart *model.Cart) *cartv1.Cart {
	if cart == nil {
		return nil
	}

	items := make([]*cartv1.CartItem, 0, len(cart.Items))
	for _, item := range cart.Items {
		if item == nil {
			continue
		}
		items = append(items, &cartv1.CartItem{
			ProductId:   item.ProductID,
			ProductName: item.ProductName,
			UnitPrice:   item.UnitPrice,
			Quantity:    item.Quantity,
			Subtotal:    item.Subtotal,
		})
	}

	return &cartv1.Cart{
		UserId:     cart.UserID,
		Items:      items,
		TotalPrice: cart.TotalPrice,
		ItemCount:  cart.ItemCount,
	}
}
