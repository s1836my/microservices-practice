package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/yourorg/micromart/services/product/internal/model"
)

// ListFilter holds optional filtering parameters for listing products.
type ListFilter struct {
	CategoryID *uuid.UUID
	SellerID   *uuid.UUID
	Page       int32
	PageSize   int32
}

// ProductRepository defines all data access operations for the product service.
type ProductRepository interface {
	// Product CRUD
	Create(ctx context.Context, product *model.Product, initialStock int32) (*model.Product, error)
	FindByID(ctx context.Context, id uuid.UUID) (*model.Product, error)
	Update(ctx context.Context, product *model.Product) (*model.Product, error)
	SoftDelete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, filter ListFilter) ([]*model.Product, int32, error)

	// Inventory
	GetInventory(ctx context.Context, productID uuid.UUID) (*model.Inventory, error)
	ListInventories(ctx context.Context, productIDs []uuid.UUID) ([]*model.Inventory, error)
	ReserveInventory(ctx context.Context, orderID uuid.UUID, items []model.InventoryItem) (bool, string, []model.InventoryItem, error)
	ReleaseInventory(ctx context.Context, orderID uuid.UUID, items []model.InventoryItem) error

	// Outbox
	ListUnpublishedEvents(ctx context.Context, limit int) ([]*model.OutboxEvent, error)
	MarkEventPublished(ctx context.Context, id uuid.UUID) error
}
