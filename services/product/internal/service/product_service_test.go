package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apperrors "github.com/yourorg/micromart/pkg/errors"
	"github.com/yourorg/micromart/services/product/internal/model"
	"github.com/yourorg/micromart/services/product/internal/repository"
	"github.com/yourorg/micromart/services/product/internal/service"
)

// --- mock repository ---

type mockProductRepo struct {
	products   map[uuid.UUID]*model.Product
	inventories map[uuid.UUID]*model.Inventory
	outbox     []*model.OutboxEvent
}

func newMockRepo() *mockProductRepo {
	return &mockProductRepo{
		products:    make(map[uuid.UUID]*model.Product),
		inventories: make(map[uuid.UUID]*model.Inventory),
	}
}

func (m *mockProductRepo) Create(_ context.Context, p *model.Product, initialStock int32) (*model.Product, error) {
	created := &model.Product{
		ID:          p.ID,
		SellerID:    p.SellerID,
		CategoryID:  p.CategoryID,
		Name:        p.Name,
		Description: p.Description,
		Price:       p.Price,
		Status:      p.Status,
		Images:      p.Images,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	m.products[created.ID] = created
	m.inventories[created.ID] = &model.Inventory{
		ProductID: created.ID,
		Stock:     initialStock,
		UpdatedAt: time.Now(),
	}
	return created, nil
}

func (m *mockProductRepo) FindByID(_ context.Context, id uuid.UUID) (*model.Product, error) {
	p, ok := m.products[id]
	if !ok || p.Status == model.ProductStatusDeleted {
		return nil, apperrors.NewNotFound("product not found")
	}
	return p, nil
}

func (m *mockProductRepo) Update(_ context.Context, p *model.Product) (*model.Product, error) {
	existing, ok := m.products[p.ID]
	if !ok || existing.Status == model.ProductStatusDeleted {
		return nil, apperrors.NewNotFound("product not found")
	}
	updated := &model.Product{
		ID:          p.ID,
		SellerID:    existing.SellerID,
		CategoryID:  existing.CategoryID,
		Name:        p.Name,
		Description: p.Description,
		Price:       p.Price,
		Status:      p.Status,
		Images:      p.Images,
		CreatedAt:   existing.CreatedAt,
		UpdatedAt:   time.Now(),
	}
	m.products[p.ID] = updated
	return updated, nil
}

func (m *mockProductRepo) SoftDelete(_ context.Context, id uuid.UUID) error {
	p, ok := m.products[id]
	if !ok || p.Status == model.ProductStatusDeleted {
		return apperrors.NewNotFound("product not found")
	}
	p.Status = model.ProductStatusDeleted
	return nil
}

func (m *mockProductRepo) List(_ context.Context, filter repository.ListFilter) ([]*model.Product, int32, error) {
	var result []*model.Product
	for _, p := range m.products {
		if p.Status == model.ProductStatusDeleted {
			continue
		}
		if filter.CategoryID != nil && p.CategoryID != *filter.CategoryID {
			continue
		}
		if filter.SellerID != nil && p.SellerID != *filter.SellerID {
			continue
		}
		result = append(result, p)
	}
	return result, int32(len(result)), nil
}

func (m *mockProductRepo) GetInventory(_ context.Context, productID uuid.UUID) (*model.Inventory, error) {
	inv, ok := m.inventories[productID]
	if !ok {
		return nil, apperrors.NewNotFound("inventory not found")
	}
	return inv, nil
}

func (m *mockProductRepo) ListInventories(_ context.Context, productIDs []uuid.UUID) ([]*model.Inventory, error) {
	var result []*model.Inventory
	for _, id := range productIDs {
		if inv, ok := m.inventories[id]; ok {
			result = append(result, inv)
		}
	}
	return result, nil
}

func (m *mockProductRepo) ReserveInventory(_ context.Context, _ uuid.UUID, items []model.InventoryItem) (bool, string, []model.InventoryItem, error) {
	for _, item := range items {
		inv, ok := m.inventories[item.ProductID]
		if !ok {
			return false, "product not found", nil, nil
		}
		if inv.Available() < item.Quantity {
			return false, "insufficient stock", nil, nil
		}
	}
	for _, item := range items {
		m.inventories[item.ProductID].ReservedStock += item.Quantity
	}
	return true, "", items, nil
}

func (m *mockProductRepo) ReleaseInventory(_ context.Context, _ uuid.UUID, items []model.InventoryItem) error {
	for _, item := range items {
		if inv, ok := m.inventories[item.ProductID]; ok {
			if inv.ReservedStock >= item.Quantity {
				inv.ReservedStock -= item.Quantity
			} else {
				inv.ReservedStock = 0
			}
			if inv.Stock >= item.Quantity {
				inv.Stock -= item.Quantity
			} else {
				inv.Stock = 0
			}
		}
	}
	return nil
}

func (m *mockProductRepo) ListUnpublishedEvents(_ context.Context, _ int) ([]*model.OutboxEvent, error) {
	return m.outbox, nil
}

func (m *mockProductRepo) MarkEventPublished(_ context.Context, id uuid.UUID) error {
	for _, e := range m.outbox {
		if e.ID == id {
			e.Published = true
		}
	}
	return nil
}

// --- helpers ---

func newSampleInput() service.CreateInput {
	return service.CreateInput{
		SellerID:     uuid.New(),
		CategoryID:   uuid.New(),
		Name:         "Widget Pro",
		Description:  "A great widget",
		Price:        1999,
		InitialStock: 50,
		Images:       []string{"https://cdn.example.com/img/1.jpg"},
	}
}

// --- tests ---

func TestProductService_Create_Success(t *testing.T) {
	repo := newMockRepo()
	svc := service.NewProductService(repo)

	in := newSampleInput()
	product, inv, err := svc.Create(context.Background(), in)

	require.NoError(t, err)
	assert.Equal(t, in.Name, product.Name)
	assert.Equal(t, in.Price, product.Price)
	assert.Equal(t, model.ProductStatusActive, product.Status)
	assert.Equal(t, int32(50), inv.Stock)
}

func TestProductService_Create_InvalidName(t *testing.T) {
	svc := service.NewProductService(newMockRepo())

	in := newSampleInput()
	in.Name = ""
	_, _, err := svc.Create(context.Background(), in)

	require.Error(t, err)
	var appErr *apperrors.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeInvalidInput, appErr.Code)
}

func TestProductService_Create_NegativePrice(t *testing.T) {
	svc := service.NewProductService(newMockRepo())

	in := newSampleInput()
	in.Price = -1
	_, _, err := svc.Create(context.Background(), in)

	require.Error(t, err)
	var appErr *apperrors.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeInvalidInput, appErr.Code)
}

func TestProductService_Create_MissingSellerID(t *testing.T) {
	svc := service.NewProductService(newMockRepo())

	in := newSampleInput()
	in.SellerID = uuid.Nil
	_, _, err := svc.Create(context.Background(), in)

	require.Error(t, err)
}

func TestProductService_Get_Success(t *testing.T) {
	repo := newMockRepo()
	svc := service.NewProductService(repo)

	created, _, err := svc.Create(context.Background(), newSampleInput())
	require.NoError(t, err)

	got, inv, err := svc.Get(context.Background(), created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, int32(50), inv.Stock)
}

func TestProductService_Get_NotFound(t *testing.T) {
	svc := service.NewProductService(newMockRepo())

	_, _, err := svc.Get(context.Background(), uuid.New())
	require.Error(t, err)
	var appErr *apperrors.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

func TestProductService_Update_Success(t *testing.T) {
	repo := newMockRepo()
	svc := service.NewProductService(repo)

	created, _, err := svc.Create(context.Background(), newSampleInput())
	require.NoError(t, err)

	updated, inv, err := svc.Update(context.Background(), service.UpdateInput{
		ProductID:   created.ID,
		Name:        "Updated Widget",
		Description: "Better than ever",
		Price:       2999,
	})
	require.NoError(t, err)
	assert.Equal(t, "Updated Widget", updated.Name)
	assert.Equal(t, int64(2999), updated.Price)
	assert.Equal(t, int32(50), inv.Stock)
}

func TestProductService_Update_SetDeletedStatus(t *testing.T) {
	repo := newMockRepo()
	svc := service.NewProductService(repo)

	created, _, err := svc.Create(context.Background(), newSampleInput())
	require.NoError(t, err)

	_, _, err = svc.Update(context.Background(), service.UpdateInput{
		ProductID: created.ID,
		Name:      "Widget",
		Price:     100,
		Status:    model.ProductStatusDeleted,
	})
	require.Error(t, err)
}

func TestProductService_Delete_Success(t *testing.T) {
	repo := newMockRepo()
	svc := service.NewProductService(repo)

	created, _, err := svc.Create(context.Background(), newSampleInput())
	require.NoError(t, err)

	err = svc.Delete(context.Background(), created.ID)
	require.NoError(t, err)

	_, _, err = svc.Get(context.Background(), created.ID)
	require.Error(t, err)
}

func TestProductService_Delete_NotFound(t *testing.T) {
	svc := service.NewProductService(newMockRepo())

	err := svc.Delete(context.Background(), uuid.New())
	require.Error(t, err)
}

func TestProductService_List(t *testing.T) {
	repo := newMockRepo()
	svc := service.NewProductService(repo)

	for i := 0; i < 3; i++ {
		_, _, err := svc.Create(context.Background(), newSampleInput())
		require.NoError(t, err)
	}

	products, invs, total, err := svc.List(context.Background(), service.ListInput{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, 3, len(products))
	assert.Equal(t, int32(3), total)
	assert.Equal(t, len(products), len(invs))
}

func TestProductService_ReserveInventory_Success(t *testing.T) {
	repo := newMockRepo()
	svc := service.NewProductService(repo)

	created, _, err := svc.Create(context.Background(), newSampleInput())
	require.NoError(t, err)

	items := []model.InventoryItem{{ProductID: created.ID, Quantity: 5}}
	success, reason, reserved, err := svc.ReserveInventory(context.Background(), uuid.New(), items)
	require.NoError(t, err)
	assert.True(t, success)
	assert.Empty(t, reason)
	assert.Len(t, reserved, 1)
}

func TestProductService_ReserveInventory_InsufficientStock(t *testing.T) {
	repo := newMockRepo()
	svc := service.NewProductService(repo)

	created, _, err := svc.Create(context.Background(), newSampleInput())
	require.NoError(t, err)

	items := []model.InventoryItem{{ProductID: created.ID, Quantity: 999}}
	success, reason, _, err := svc.ReserveInventory(context.Background(), uuid.New(), items)
	require.NoError(t, err)
	assert.False(t, success)
	assert.NotEmpty(t, reason)
}

func TestProductService_ReleaseInventory(t *testing.T) {
	repo := newMockRepo()
	svc := service.NewProductService(repo)

	created, _, err := svc.Create(context.Background(), newSampleInput())
	require.NoError(t, err)

	orderID := uuid.New()
	items := []model.InventoryItem{{ProductID: created.ID, Quantity: 10}}

	_, _, _, err = svc.ReserveInventory(context.Background(), orderID, items)
	require.NoError(t, err)

	err = svc.ReleaseInventory(context.Background(), orderID, items)
	require.NoError(t, err)

	inv, err := svc.GetInventory(context.Background(), created.ID)
	require.NoError(t, err)
	assert.Equal(t, int32(0), inv.ReservedStock)
}

func TestProductService_GetInventory(t *testing.T) {
	repo := newMockRepo()
	svc := service.NewProductService(repo)

	created, _, err := svc.Create(context.Background(), newSampleInput())
	require.NoError(t, err)

	inv, err := svc.GetInventory(context.Background(), created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, inv.ProductID)
	assert.Equal(t, int32(50), inv.Stock)
}

func TestProductService_Create_InvalidDescription(t *testing.T) {
	svc := service.NewProductService(newMockRepo())

	in := newSampleInput()
	in.Description = string(make([]byte, 5001))
	_, _, err := svc.Create(context.Background(), in)

	require.Error(t, err)
	var appErr *apperrors.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeInvalidInput, appErr.Code)
}

func TestProductService_Create_NegativeStock(t *testing.T) {
	svc := service.NewProductService(newMockRepo())

	in := newSampleInput()
	in.InitialStock = -1
	_, _, err := svc.Create(context.Background(), in)

	require.Error(t, err)
}

func TestProductService_Create_NilImages(t *testing.T) {
	repo := newMockRepo()
	svc := service.NewProductService(repo)

	in := newSampleInput()
	in.Images = nil
	product, inv, err := svc.Create(context.Background(), in)

	require.NoError(t, err)
	assert.NotNil(t, product.Images)
	assert.Equal(t, int32(50), inv.Stock)
}

func TestProductService_Update_InvalidDescription(t *testing.T) {
	repo := newMockRepo()
	svc := service.NewProductService(repo)

	created, _, err := svc.Create(context.Background(), newSampleInput())
	require.NoError(t, err)

	_, _, err = svc.Update(context.Background(), service.UpdateInput{
		ProductID:   created.ID,
		Name:        "Widget",
		Description: string(make([]byte, 5001)),
		Price:       100,
	})
	require.Error(t, err)
}

func TestProductService_Update_NegativePrice(t *testing.T) {
	repo := newMockRepo()
	svc := service.NewProductService(repo)

	created, _, err := svc.Create(context.Background(), newSampleInput())
	require.NoError(t, err)

	_, _, err = svc.Update(context.Background(), service.UpdateInput{
		ProductID: created.ID,
		Name:      "Widget",
		Price:     -1,
	})
	require.Error(t, err)
}

func TestProductService_Update_NotFound(t *testing.T) {
	svc := service.NewProductService(newMockRepo())

	_, _, err := svc.Update(context.Background(), service.UpdateInput{
		ProductID: uuid.New(),
		Name:      "Widget",
		Price:     100,
	})
	require.Error(t, err)
	var appErr *apperrors.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

func TestProductService_List_WithCategoryFilter(t *testing.T) {
	repo := newMockRepo()
	svc := service.NewProductService(repo)

	catID := uuid.New()
	in := newSampleInput()
	in.CategoryID = catID
	_, _, err := svc.Create(context.Background(), in)
	require.NoError(t, err)

	otherIn := newSampleInput()
	_, _, err = svc.Create(context.Background(), otherIn)
	require.NoError(t, err)

	products, invs, total, err := svc.List(context.Background(), service.ListInput{
		CategoryID: &catID,
		Page:       1,
		PageSize:   10,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, len(products))
	assert.Equal(t, int32(1), total)
	assert.Equal(t, len(products), len(invs))
}
