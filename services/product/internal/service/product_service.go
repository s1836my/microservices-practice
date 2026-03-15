package service

import (
	"context"

	"github.com/google/uuid"
	apperrors "github.com/yourorg/micromart/pkg/errors"
	"github.com/yourorg/micromart/services/product/internal/model"
	"github.com/yourorg/micromart/services/product/internal/repository"
	"github.com/yourorg/micromart/services/product/internal/validator"
)

// CreateInput holds the parameters for creating a new product.
type CreateInput struct {
	SellerID     uuid.UUID
	CategoryID   uuid.UUID
	Name         string
	Description  string
	Price        int64
	InitialStock int32
	Images       []string
}

// UpdateInput holds the parameters for updating an existing product.
type UpdateInput struct {
	ProductID   uuid.UUID
	Name        string
	Description string
	Price       int64
	Status      model.ProductStatus
}

// ListInput holds filtering and pagination parameters for listing products.
type ListInput struct {
	CategoryID *uuid.UUID
	SellerID   *uuid.UUID
	Page       int32
	PageSize   int32
}

// ProductService defines the business operations for the product domain.
type ProductService interface {
	Create(ctx context.Context, in CreateInput) (*model.Product, *model.Inventory, error)
	Get(ctx context.Context, productID uuid.UUID) (*model.Product, *model.Inventory, error)
	Update(ctx context.Context, in UpdateInput) (*model.Product, *model.Inventory, error)
	Delete(ctx context.Context, productID uuid.UUID) error
	List(ctx context.Context, in ListInput) ([]*model.Product, []*model.Inventory, int32, error)
	ReserveInventory(ctx context.Context, orderID uuid.UUID, items []model.InventoryItem) (bool, string, []model.InventoryItem, error)
	ReleaseInventory(ctx context.Context, orderID uuid.UUID, items []model.InventoryItem) error
	GetInventory(ctx context.Context, productID uuid.UUID) (*model.Inventory, error)
}

type productService struct {
	repo repository.ProductRepository
}

// NewProductService creates a new ProductService.
func NewProductService(repo repository.ProductRepository) ProductService {
	return &productService{repo: repo}
}

func (s *productService) Create(ctx context.Context, in CreateInput) (*model.Product, *model.Inventory, error) {
	if err := validator.ValidateName(in.Name); err != nil {
		return nil, nil, err
	}
	if err := validator.ValidateDescription(in.Description); err != nil {
		return nil, nil, err
	}
	if err := validator.ValidatePrice(in.Price); err != nil {
		return nil, nil, err
	}
	if err := validator.ValidateStock(in.InitialStock); err != nil {
		return nil, nil, err
	}
	if in.SellerID == uuid.Nil {
		return nil, nil, apperrors.NewInvalidInput("seller_id is required")
	}
	if in.CategoryID == uuid.Nil {
		return nil, nil, apperrors.NewInvalidInput("category_id is required")
	}

	images := in.Images
	if images == nil {
		images = []string{}
	}

	product := &model.Product{
		ID:          uuid.New(),
		SellerID:    in.SellerID,
		CategoryID:  in.CategoryID,
		Name:        in.Name,
		Description: in.Description,
		Price:       in.Price,
		Status:      model.ProductStatusActive,
		Images:      images,
	}

	created, err := s.repo.Create(ctx, product, in.InitialStock)
	if err != nil {
		return nil, nil, err
	}

	inv, err := s.repo.GetInventory(ctx, created.ID)
	if err != nil {
		return nil, nil, err
	}

	return created, inv, nil
}

func (s *productService) Get(ctx context.Context, productID uuid.UUID) (*model.Product, *model.Inventory, error) {
	product, err := s.repo.FindByID(ctx, productID)
	if err != nil {
		return nil, nil, err
	}

	inv, err := s.repo.GetInventory(ctx, productID)
	if err != nil {
		return nil, nil, err
	}

	return product, inv, nil
}

func (s *productService) Update(ctx context.Context, in UpdateInput) (*model.Product, *model.Inventory, error) {
	if err := validator.ValidateName(in.Name); err != nil {
		return nil, nil, err
	}
	if err := validator.ValidateDescription(in.Description); err != nil {
		return nil, nil, err
	}
	if err := validator.ValidatePrice(in.Price); err != nil {
		return nil, nil, err
	}

	existing, err := s.repo.FindByID(ctx, in.ProductID)
	if err != nil {
		return nil, nil, err
	}

	status := in.Status
	if status == "" {
		status = existing.Status
	}
	if status == model.ProductStatusDeleted {
		return nil, nil, apperrors.NewInvalidInput("cannot set status to deleted via update; use delete endpoint")
	}

	toUpdate := &model.Product{
		ID:          existing.ID,
		SellerID:    existing.SellerID,
		CategoryID:  existing.CategoryID,
		Name:        in.Name,
		Description: in.Description,
		Price:       in.Price,
		Status:      status,
		Images:      existing.Images,
	}

	updated, err := s.repo.Update(ctx, toUpdate)
	if err != nil {
		return nil, nil, err
	}

	inv, err := s.repo.GetInventory(ctx, updated.ID)
	if err != nil {
		return nil, nil, err
	}

	return updated, inv, nil
}

func (s *productService) Delete(ctx context.Context, productID uuid.UUID) error {
	return s.repo.SoftDelete(ctx, productID)
}

func (s *productService) List(ctx context.Context, in ListInput) ([]*model.Product, []*model.Inventory, int32, error) {
	products, total, err := s.repo.List(ctx, repository.ListFilter{
		CategoryID: in.CategoryID,
		SellerID:   in.SellerID,
		Page:       validator.ValidatePage(in.Page),
		PageSize:   validator.ValidatePageSize(in.PageSize),
	})
	if err != nil {
		return nil, nil, 0, err
	}
	if len(products) == 0 {
		return products, nil, total, nil
	}

	productIDs := make([]uuid.UUID, len(products))
	for i, p := range products {
		productIDs[i] = p.ID
	}

	inventories, err := s.repo.ListInventories(ctx, productIDs)
	if err != nil {
		return nil, nil, 0, err
	}

	// Build inventory map for ordered assembly
	invMap := make(map[uuid.UUID]*model.Inventory, len(inventories))
	for _, inv := range inventories {
		invMap[inv.ProductID] = inv
	}

	orderedInvs := make([]*model.Inventory, len(products))
	for i, p := range products {
		inv := invMap[p.ID]
		if inv == nil {
			inv = &model.Inventory{ProductID: p.ID}
		}
		orderedInvs[i] = inv
	}

	return products, orderedInvs, total, nil
}

func (s *productService) ReserveInventory(
	ctx context.Context,
	orderID uuid.UUID,
	items []model.InventoryItem,
) (bool, string, []model.InventoryItem, error) {
	return s.repo.ReserveInventory(ctx, orderID, items)
}

func (s *productService) ReleaseInventory(ctx context.Context, orderID uuid.UUID, items []model.InventoryItem) error {
	return s.repo.ReleaseInventory(ctx, orderID, items)
}

func (s *productService) GetInventory(ctx context.Context, productID uuid.UUID) (*model.Inventory, error) {
	return s.repo.GetInventory(ctx, productID)
}
