package service

import (
	"context"
	"strings"

	apperrors "github.com/yourorg/micromart/pkg/errors"
	"github.com/yourorg/micromart/services/cart/internal/model"
	"github.com/yourorg/micromart/services/cart/internal/repository"
)

type CartService interface {
	GetCart(ctx context.Context, userID string) (*model.Cart, error)
	AddItem(ctx context.Context, userID, productID string, quantity int32) (*model.Cart, error)
	UpdateItem(ctx context.Context, userID, productID string, quantity int32) (*model.Cart, error)
	RemoveItem(ctx context.Context, userID, productID string) error
	ClearCart(ctx context.Context, userID string) error
}

type cartService struct {
	repo repository.CartRepository
}

func NewCartService(repo repository.CartRepository) CartService {
	return &cartService{repo: repo}
}

func (s *cartService) GetCart(ctx context.Context, userID string) (*model.Cart, error) {
	if err := validateUserID(userID); err != nil {
		return nil, err
	}

	items, err := s.repo.GetCart(ctx, userID)
	if err != nil {
		return nil, err
	}
	return model.NewCart(userID, items), nil
}

func (s *cartService) AddItem(ctx context.Context, userID, productID string, quantity int32) (*model.Cart, error) {
	if err := validateIdentifiers(userID, productID); err != nil {
		return nil, err
	}
	if quantity <= 0 {
		return nil, apperrors.NewInvalidInput("quantity must be greater than 0")
	}

	items, err := s.repo.GetCart(ctx, userID)
	if err != nil {
		return nil, err
	}

	item := findItem(items, productID)
	if item == nil {
		item = &model.CartItem{ProductID: productID, Quantity: quantity}
		items = append(items, item)
	} else {
		item.Quantity += quantity
	}

	if err := s.repo.UpsertItem(ctx, userID, item); err != nil {
		return nil, err
	}
	return model.NewCart(userID, items), nil
}

func (s *cartService) UpdateItem(ctx context.Context, userID, productID string, quantity int32) (*model.Cart, error) {
	if err := validateIdentifiers(userID, productID); err != nil {
		return nil, err
	}
	if quantity < 0 {
		return nil, apperrors.NewInvalidInput("quantity must be greater than or equal to 0")
	}

	items, err := s.repo.GetCart(ctx, userID)
	if err != nil {
		return nil, err
	}

	idx := findItemIndex(items, productID)
	if idx == -1 {
		return nil, apperrors.NewNotFound("cart item not found")
	}

	if quantity == 0 {
		if err := s.repo.RemoveItem(ctx, userID, productID); err != nil {
			return nil, err
		}
		items = append(items[:idx], items[idx+1:]...)
		return model.NewCart(userID, items), nil
	}

	items[idx].Quantity = quantity
	if err := s.repo.UpsertItem(ctx, userID, items[idx]); err != nil {
		return nil, err
	}
	return model.NewCart(userID, items), nil
}

func (s *cartService) RemoveItem(ctx context.Context, userID, productID string) error {
	if err := validateIdentifiers(userID, productID); err != nil {
		return err
	}
	return s.repo.RemoveItem(ctx, userID, productID)
}

func (s *cartService) ClearCart(ctx context.Context, userID string) error {
	if err := validateUserID(userID); err != nil {
		return err
	}
	return s.repo.ClearCart(ctx, userID)
}

func validateIdentifiers(userID, productID string) error {
	if err := validateUserID(userID); err != nil {
		return err
	}
	if strings.TrimSpace(productID) == "" {
		return apperrors.NewInvalidInput("product_id is required")
	}
	return nil
}

func validateUserID(userID string) error {
	if strings.TrimSpace(userID) == "" {
		return apperrors.NewInvalidInput("user_id is required")
	}
	return nil
}

func findItem(items []*model.CartItem, productID string) *model.CartItem {
	for _, item := range items {
		if item != nil && item.ProductID == productID {
			return item
		}
	}
	return nil
}

func findItemIndex(items []*model.CartItem, productID string) int {
	for i, item := range items {
		if item != nil && item.ProductID == productID {
			return i
		}
	}
	return -1
}
