package model

import (
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	orderv1 "github.com/yourorg/micromart/proto/order/v1"
)

type OrderStatus string

const (
	OrderStatusCreated            OrderStatus = "CREATED"
	OrderStatusPaymentPending     OrderStatus = "PAYMENT_PENDING"
	OrderStatusPaymentCompleted   OrderStatus = "PAYMENT_COMPLETED"
	OrderStatusInventoryReserving OrderStatus = "INVENTORY_RESERVING"
	OrderStatusCompleted          OrderStatus = "COMPLETED"
	OrderStatusCancelled          OrderStatus = "CANCELLED"
	OrderStatusCompensating       OrderStatus = "COMPENSATING"
)

type SagaProgress string

const (
	SagaProgressPending   SagaProgress = "PENDING"
	SagaProgressCompleted SagaProgress = "COMPLETED"
	SagaProgressFailed    SagaProgress = "FAILED"
	SagaProgressReserved  SagaProgress = "RESERVED"
)

type Order struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	Status         OrderStatus
	TotalAmount    int64
	Currency       string
	Items          []OrderItem
	FailureReason  string
	IdempotencyKey string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type OrderItem struct {
	ID          uuid.UUID
	OrderID     uuid.UUID
	ProductID   uuid.UUID
	ProductName string
	SellerID    uuid.UUID
	UnitPrice   int64
	Quantity    int32
	Subtotal    int64
}

type SagaState struct {
	OrderID            uuid.UUID
	PaymentStatus      SagaProgress
	InventoryStatus    SagaProgress
	CompensationStatus string
	LastEventType      string
	UpdatedAt          time.Time
}

func (o *Order) ToProto() *orderv1.Order {
	items := make([]*orderv1.OrderItem, 0, len(o.Items))
	for _, item := range o.Items {
		items = append(items, item.ToProto())
	}

	return &orderv1.Order{
		OrderId:       o.ID.String(),
		UserId:        o.UserID.String(),
		Status:        string(o.Status),
		TotalAmount:   o.TotalAmount,
		Currency:      o.Currency,
		Items:         items,
		FailureReason: o.FailureReason,
		CreatedAt:     timestamppb.New(o.CreatedAt),
		UpdatedAt:     timestamppb.New(o.UpdatedAt),
	}
}

func (i *OrderItem) ToProto() *orderv1.OrderItem {
	return &orderv1.OrderItem{
		ProductId:   i.ProductID.String(),
		ProductName: i.ProductName,
		SellerId:    i.SellerID.String(),
		UnitPrice:   i.UnitPrice,
		Quantity:    i.Quantity,
		Subtotal:    i.Subtotal,
	}
}
