package handler_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apperrors "github.com/yourorg/micromart/pkg/errors"
	productv1 "github.com/yourorg/micromart/proto/product/v1"
	"github.com/yourorg/micromart/services/product/internal/handler"
	"github.com/yourorg/micromart/services/product/internal/model"
	"github.com/yourorg/micromart/services/product/internal/service"
)

// --- mock service ---

type mockProductService struct {
	createFn          func(ctx context.Context, in service.CreateInput) (*model.Product, *model.Inventory, error)
	getFn             func(ctx context.Context, id uuid.UUID) (*model.Product, *model.Inventory, error)
	updateFn          func(ctx context.Context, in service.UpdateInput) (*model.Product, *model.Inventory, error)
	deleteFn          func(ctx context.Context, id uuid.UUID) error
	listFn            func(ctx context.Context, in service.ListInput) ([]*model.Product, []*model.Inventory, int32, error)
	reserveFn         func(ctx context.Context, orderID uuid.UUID, items []model.InventoryItem) (bool, string, []model.InventoryItem, error)
	releaseFn         func(ctx context.Context, orderID uuid.UUID, items []model.InventoryItem) error
	getInventoryFn    func(ctx context.Context, productID uuid.UUID) (*model.Inventory, error)
}

func (m *mockProductService) Create(ctx context.Context, in service.CreateInput) (*model.Product, *model.Inventory, error) {
	return m.createFn(ctx, in)
}
func (m *mockProductService) Get(ctx context.Context, id uuid.UUID) (*model.Product, *model.Inventory, error) {
	return m.getFn(ctx, id)
}
func (m *mockProductService) Update(ctx context.Context, in service.UpdateInput) (*model.Product, *model.Inventory, error) {
	return m.updateFn(ctx, in)
}
func (m *mockProductService) Delete(ctx context.Context, id uuid.UUID) error {
	return m.deleteFn(ctx, id)
}
func (m *mockProductService) List(ctx context.Context, in service.ListInput) ([]*model.Product, []*model.Inventory, int32, error) {
	return m.listFn(ctx, in)
}
func (m *mockProductService) ReserveInventory(ctx context.Context, orderID uuid.UUID, items []model.InventoryItem) (bool, string, []model.InventoryItem, error) {
	return m.reserveFn(ctx, orderID, items)
}
func (m *mockProductService) ReleaseInventory(ctx context.Context, orderID uuid.UUID, items []model.InventoryItem) error {
	return m.releaseFn(ctx, orderID, items)
}
func (m *mockProductService) GetInventory(ctx context.Context, productID uuid.UUID) (*model.Inventory, error) {
	return m.getInventoryFn(ctx, productID)
}

// --- helpers ---

func sampleProduct() *model.Product {
	return &model.Product{
		ID:          uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"),
		SellerID:    uuid.MustParse("550e8400-e29b-41d4-a716-446655440001"),
		CategoryID:  uuid.MustParse("550e8400-e29b-41d4-a716-446655440002"),
		Name:        "Widget Pro",
		Description: "A great widget",
		Price:       1999,
		Status:      model.ProductStatusActive,
		Images:      []string{"https://cdn.example.com/img/1.jpg"},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

func sampleInventory(productID uuid.UUID) *model.Inventory {
	return &model.Inventory{
		ProductID:     productID,
		Stock:         100,
		ReservedStock: 10,
		UpdatedAt:     time.Now(),
	}
}

// --- tests ---

func TestProductHandler_CreateProduct(t *testing.T) {
	p := sampleProduct()
	inv := sampleInventory(p.ID)
	svc := &mockProductService{
		createFn: func(_ context.Context, _ service.CreateInput) (*model.Product, *model.Inventory, error) {
			return p, inv, nil
		},
	}
	h := handler.NewProductHandler(svc)

	resp, err := h.CreateProduct(context.Background(), &productv1.CreateProductRequest{
		SellerId:     p.SellerID.String(),
		CategoryId:   p.CategoryID.String(),
		Name:         p.Name,
		Description:  p.Description,
		Price:        p.Price,
		InitialStock: 100,
	})
	require.NoError(t, err)
	assert.Equal(t, p.ID.String(), resp.Product.ProductId)
	assert.Equal(t, int32(100), resp.Product.Stock)
}

func TestProductHandler_CreateProduct_InvalidSellerID(t *testing.T) {
	h := handler.NewProductHandler(&mockProductService{})

	_, err := h.CreateProduct(context.Background(), &productv1.CreateProductRequest{
		SellerId:   "not-a-uuid",
		CategoryId: uuid.New().String(),
		Name:       "Test",
	})
	assert.Error(t, err)
}

func TestProductHandler_GetProduct(t *testing.T) {
	p := sampleProduct()
	inv := sampleInventory(p.ID)
	svc := &mockProductService{
		getFn: func(_ context.Context, id uuid.UUID) (*model.Product, *model.Inventory, error) {
			if id == p.ID {
				return p, inv, nil
			}
			return nil, nil, apperrors.NewNotFound("not found")
		},
	}
	h := handler.NewProductHandler(svc)

	resp, err := h.GetProduct(context.Background(), &productv1.GetProductRequest{ProductId: p.ID.String()})
	require.NoError(t, err)
	assert.Equal(t, p.ID.String(), resp.Product.ProductId)
	assert.Equal(t, int32(100), resp.Product.Stock)
}

func TestProductHandler_GetProduct_InvalidUUID(t *testing.T) {
	h := handler.NewProductHandler(&mockProductService{})

	_, err := h.GetProduct(context.Background(), &productv1.GetProductRequest{ProductId: "bad-uuid"})
	assert.Error(t, err)
}

func TestProductHandler_UpdateProduct(t *testing.T) {
	p := sampleProduct()
	inv := sampleInventory(p.ID)
	svc := &mockProductService{
		updateFn: func(_ context.Context, in service.UpdateInput) (*model.Product, *model.Inventory, error) {
			updated := *p
			updated.Name = in.Name
			updated.Price = in.Price
			return &updated, inv, nil
		},
	}
	h := handler.NewProductHandler(svc)

	resp, err := h.UpdateProduct(context.Background(), &productv1.UpdateProductRequest{
		ProductId:   p.ID.String(),
		Name:        "New Name",
		Description: "New Desc",
		Price:       2999,
	})
	require.NoError(t, err)
	assert.Equal(t, "New Name", resp.Product.Name)
	assert.Equal(t, int64(2999), resp.Product.Price)
}

func TestProductHandler_DeleteProduct(t *testing.T) {
	called := false
	svc := &mockProductService{
		deleteFn: func(_ context.Context, _ uuid.UUID) error {
			called = true
			return nil
		},
	}
	h := handler.NewProductHandler(svc)

	_, err := h.DeleteProduct(context.Background(), &productv1.DeleteProductRequest{
		ProductId: uuid.New().String(),
	})
	require.NoError(t, err)
	assert.True(t, called)
}

func TestProductHandler_ListProducts(t *testing.T) {
	p := sampleProduct()
	inv := sampleInventory(p.ID)
	svc := &mockProductService{
		listFn: func(_ context.Context, _ service.ListInput) ([]*model.Product, []*model.Inventory, int32, error) {
			return []*model.Product{p}, []*model.Inventory{inv}, 1, nil
		},
	}
	h := handler.NewProductHandler(svc)

	resp, err := h.ListProducts(context.Background(), &productv1.ListProductsRequest{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Len(t, resp.Products, 1)
	assert.Equal(t, int32(1), resp.Total)
	assert.Equal(t, int32(100), resp.Products[0].Stock)
}

func TestProductHandler_ReserveInventory(t *testing.T) {
	productID := uuid.New()
	svc := &mockProductService{
		reserveFn: func(_ context.Context, _ uuid.UUID, items []model.InventoryItem) (bool, string, []model.InventoryItem, error) {
			return true, "", items, nil
		},
	}
	h := handler.NewProductHandler(svc)

	resp, err := h.ReserveInventory(context.Background(), &productv1.ReserveInventoryRequest{
		OrderId: uuid.New().String(),
		Items:   []*productv1.InventoryItem{{ProductId: productID.String(), Quantity: 5}},
	})
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Len(t, resp.ReservedItems, 1)
}

func TestProductHandler_ReleaseInventory(t *testing.T) {
	called := false
	svc := &mockProductService{
		releaseFn: func(_ context.Context, _ uuid.UUID, _ []model.InventoryItem) error {
			called = true
			return nil
		},
	}
	h := handler.NewProductHandler(svc)

	_, err := h.ReleaseInventory(context.Background(), &productv1.ReleaseInventoryRequest{
		OrderId: uuid.New().String(),
		Items:   []*productv1.InventoryItem{{ProductId: uuid.New().String(), Quantity: 5}},
	})
	require.NoError(t, err)
	assert.True(t, called)
}

func TestProductHandler_GetInventory(t *testing.T) {
	productID := uuid.New()
	svc := &mockProductService{
		getInventoryFn: func(_ context.Context, id uuid.UUID) (*model.Inventory, error) {
			return &model.Inventory{
				ProductID:     id,
				Stock:         100,
				ReservedStock: 20,
			}, nil
		},
	}
	h := handler.NewProductHandler(svc)

	resp, err := h.GetInventory(context.Background(), &productv1.GetInventoryRequest{ProductId: productID.String()})
	require.NoError(t, err)
	assert.Equal(t, int32(100), resp.Stock)
	assert.Equal(t, int32(20), resp.ReservedStock)
	assert.Equal(t, int32(80), resp.Available)
}
