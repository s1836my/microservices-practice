package model

import (
	"time"

	"github.com/google/uuid"
	productv1 "github.com/yourorg/micromart/proto/product/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ProductStatus string

const (
	ProductStatusDraft    ProductStatus = "draft"
	ProductStatusActive   ProductStatus = "active"
	ProductStatusInactive ProductStatus = "inactive"
	ProductStatusDeleted  ProductStatus = "deleted"
)

type Product struct {
	ID          uuid.UUID
	SellerID    uuid.UUID
	CategoryID  uuid.UUID
	Name        string
	Description string
	Price       int64
	Status      ProductStatus
	Images      []string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (p *Product) ToProto(stock int32) *productv1.Product {
	return &productv1.Product{
		ProductId:   p.ID.String(),
		SellerId:    p.SellerID.String(),
		CategoryId:  p.CategoryID.String(),
		Name:        p.Name,
		Description: p.Description,
		Price:       p.Price,
		Stock:       stock,
		Status:      string(p.Status),
		Images:      p.Images,
		CreatedAt:   timestamppb.New(p.CreatedAt),
		UpdatedAt:   timestamppb.New(p.UpdatedAt),
	}
}

type Category struct {
	ID        uuid.UUID
	Name      string
	Slug      string
	ParentID  *uuid.UUID
	SortOrder int
	CreatedAt time.Time
}

type Inventory struct {
	ProductID     uuid.UUID
	Stock         int32
	ReservedStock int32
	UpdatedAt     time.Time
}

func (inv *Inventory) Available() int32 {
	return inv.Stock - inv.ReservedStock
}

type InventoryItem struct {
	ProductID uuid.UUID
	Quantity  int32
}
