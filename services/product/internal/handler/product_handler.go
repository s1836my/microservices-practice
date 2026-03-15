package handler

import (
	"context"

	"github.com/google/uuid"
	apperrors "github.com/yourorg/micromart/pkg/errors"
	productv1 "github.com/yourorg/micromart/proto/product/v1"
	"github.com/yourorg/micromart/services/product/internal/model"
	"github.com/yourorg/micromart/services/product/internal/service"
)

// ProductHandler implements the gRPC ProductServiceServer interface.
type ProductHandler struct {
	productv1.UnimplementedProductServiceServer
	svc service.ProductService
}

// NewProductHandler creates a new ProductHandler.
func NewProductHandler(svc service.ProductService) *ProductHandler {
	return &ProductHandler{svc: svc}
}

func (h *ProductHandler) CreateProduct(ctx context.Context, req *productv1.CreateProductRequest) (*productv1.CreateProductResponse, error) {
	sellerID, err := uuid.Parse(req.SellerId)
	if err != nil {
		return nil, apperrors.NewInvalidInput("invalid seller_id")
	}
	categoryID, err := uuid.Parse(req.CategoryId)
	if err != nil {
		return nil, apperrors.NewInvalidInput("invalid category_id")
	}

	product, inv, err := h.svc.Create(ctx, service.CreateInput{
		SellerID:     sellerID,
		CategoryID:   categoryID,
		Name:         req.Name,
		Description:  req.Description,
		Price:        req.Price,
		InitialStock: req.InitialStock,
		Images:       req.Images,
	})
	if err != nil {
		return nil, err
	}

	return &productv1.CreateProductResponse{
		Product: product.ToProto(inv.Stock),
	}, nil
}

func (h *ProductHandler) GetProduct(ctx context.Context, req *productv1.GetProductRequest) (*productv1.GetProductResponse, error) {
	productID, err := uuid.Parse(req.ProductId)
	if err != nil {
		return nil, apperrors.NewInvalidInput("invalid product_id")
	}

	product, inv, err := h.svc.Get(ctx, productID)
	if err != nil {
		return nil, err
	}

	return &productv1.GetProductResponse{
		Product: product.ToProto(inv.Stock),
	}, nil
}

func (h *ProductHandler) UpdateProduct(ctx context.Context, req *productv1.UpdateProductRequest) (*productv1.UpdateProductResponse, error) {
	productID, err := uuid.Parse(req.ProductId)
	if err != nil {
		return nil, apperrors.NewInvalidInput("invalid product_id")
	}

	product, inv, err := h.svc.Update(ctx, service.UpdateInput{
		ProductID:   productID,
		Name:        req.Name,
		Description: req.Description,
		Price:       req.Price,
		Status:      model.ProductStatus(req.Status),
	})
	if err != nil {
		return nil, err
	}

	return &productv1.UpdateProductResponse{
		Product: product.ToProto(inv.Stock),
	}, nil
}

func (h *ProductHandler) DeleteProduct(ctx context.Context, req *productv1.DeleteProductRequest) (*productv1.DeleteProductResponse, error) {
	productID, err := uuid.Parse(req.ProductId)
	if err != nil {
		return nil, apperrors.NewInvalidInput("invalid product_id")
	}

	if err = h.svc.Delete(ctx, productID); err != nil {
		return nil, err
	}

	return &productv1.DeleteProductResponse{}, nil
}

func (h *ProductHandler) ListProducts(ctx context.Context, req *productv1.ListProductsRequest) (*productv1.ListProductsResponse, error) {
	in := service.ListInput{
		Page:     req.Page,
		PageSize: req.PageSize,
	}

	if req.CategoryId != "" {
		catID, err := uuid.Parse(req.CategoryId)
		if err != nil {
			return nil, apperrors.NewInvalidInput("invalid category_id")
		}
		in.CategoryID = &catID
	}

	if req.SellerId != "" {
		sellerID, err := uuid.Parse(req.SellerId)
		if err != nil {
			return nil, apperrors.NewInvalidInput("invalid seller_id")
		}
		in.SellerID = &sellerID
	}

	products, inventories, total, err := h.svc.List(ctx, in)
	if err != nil {
		return nil, err
	}

	protoProducts := make([]*productv1.Product, len(products))
	for i, p := range products {
		var stock int32
		if i < len(inventories) && inventories[i] != nil {
			stock = inventories[i].Stock
		}
		protoProducts[i] = p.ToProto(stock)
	}

	return &productv1.ListProductsResponse{
		Products: protoProducts,
		Total:    total,
	}, nil
}

func (h *ProductHandler) ReserveInventory(ctx context.Context, req *productv1.ReserveInventoryRequest) (*productv1.ReserveInventoryResponse, error) {
	orderID, err := uuid.Parse(req.OrderId)
	if err != nil {
		return nil, apperrors.NewInvalidInput("invalid order_id")
	}

	items := make([]model.InventoryItem, len(req.Items))
	for i, item := range req.Items {
		pid, err := uuid.Parse(item.ProductId)
		if err != nil {
			return nil, apperrors.NewInvalidInput("invalid product_id in items[%d]", i)
		}
		items[i] = model.InventoryItem{ProductID: pid, Quantity: item.Quantity}
	}

	success, reason, reserved, err := h.svc.ReserveInventory(ctx, orderID, items)
	if err != nil {
		return nil, err
	}

	protoItems := make([]*productv1.InventoryItem, len(reserved))
	for i, item := range reserved {
		protoItems[i] = &productv1.InventoryItem{
			ProductId: item.ProductID.String(),
			Quantity:  item.Quantity,
		}
	}

	return &productv1.ReserveInventoryResponse{
		Success:       success,
		FailureReason: reason,
		ReservedItems: protoItems,
	}, nil
}

func (h *ProductHandler) ReleaseInventory(ctx context.Context, req *productv1.ReleaseInventoryRequest) (*productv1.ReleaseInventoryResponse, error) {
	orderID, err := uuid.Parse(req.OrderId)
	if err != nil {
		return nil, apperrors.NewInvalidInput("invalid order_id")
	}

	items := make([]model.InventoryItem, len(req.Items))
	for i, item := range req.Items {
		pid, err := uuid.Parse(item.ProductId)
		if err != nil {
			return nil, apperrors.NewInvalidInput("invalid product_id in items[%d]", i)
		}
		items[i] = model.InventoryItem{ProductID: pid, Quantity: item.Quantity}
	}

	if err = h.svc.ReleaseInventory(ctx, orderID, items); err != nil {
		return nil, err
	}

	return &productv1.ReleaseInventoryResponse{}, nil
}

func (h *ProductHandler) GetInventory(ctx context.Context, req *productv1.GetInventoryRequest) (*productv1.GetInventoryResponse, error) {
	productID, err := uuid.Parse(req.ProductId)
	if err != nil {
		return nil, apperrors.NewInvalidInput("invalid product_id")
	}

	inv, err := h.svc.GetInventory(ctx, productID)
	if err != nil {
		return nil, err
	}

	return &productv1.GetInventoryResponse{
		ProductId:     inv.ProductID.String(),
		Stock:         inv.Stock,
		ReservedStock: inv.ReservedStock,
		Available:     inv.Available(),
	}, nil
}
