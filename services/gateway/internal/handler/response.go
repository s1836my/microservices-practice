package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	cartv1 "github.com/yourorg/micromart/proto/cart/v1"
	orderv1 "github.com/yourorg/micromart/proto/order/v1"
	productv1 "github.com/yourorg/micromart/proto/product/v1"
	searchv1 "github.com/yourorg/micromart/proto/search/v1"
	userv1 "github.com/yourorg/micromart/proto/user/v1"
)

// ErrorResponse is the standard error body.
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code"`
	Details any    `json:"details,omitempty"`
}

// PaginationMeta holds pagination info.
type PaginationMeta struct {
	Total    int32 `json:"total"`
	Page     int32 `json:"page"`
	PageSize int32 `json:"page_size"`
}

// UserResponse maps proto User → REST response.
type UserResponse struct {
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

// ProductResponse maps proto Product → REST response.
type ProductResponse struct {
	ProductID   string   `json:"product_id"`
	SellerID    string   `json:"seller_id"`
	CategoryID  string   `json:"category_id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Price       int64    `json:"price"`
	Stock       int32    `json:"stock"`
	Status      string   `json:"status"`
	Images      []string `json:"images"`
	CreatedAt   string   `json:"created_at,omitempty"`
	UpdatedAt   string   `json:"updated_at,omitempty"`
}

// CartItemResponse maps proto CartItem → REST response.
type CartItemResponse struct {
	ProductID   string `json:"product_id"`
	ProductName string `json:"product_name"`
	UnitPrice   int64  `json:"unit_price"`
	Quantity    int32  `json:"quantity"`
	Subtotal    int64  `json:"subtotal"`
}

// CartResponse maps proto Cart → REST response.
type CartResponse struct {
	UserID     string             `json:"user_id"`
	Items      []CartItemResponse `json:"items"`
	TotalPrice int64              `json:"total_price"`
	ItemCount  int32              `json:"item_count"`
}

// OrderItemResponse maps proto OrderItem → REST response.
type OrderItemResponse struct {
	ProductID   string `json:"product_id"`
	ProductName string `json:"product_name"`
	SellerID    string `json:"seller_id"`
	UnitPrice   int64  `json:"unit_price"`
	Quantity    int32  `json:"quantity"`
	Subtotal    int64  `json:"subtotal"`
}

// OrderResponse maps proto Order → REST response.
type OrderResponse struct {
	OrderID       string              `json:"order_id"`
	UserID        string              `json:"user_id"`
	Status        string              `json:"status"`
	TotalAmount   int64               `json:"total_amount"`
	Currency      string              `json:"currency"`
	Items         []OrderItemResponse `json:"items"`
	FailureReason string              `json:"failure_reason,omitempty"`
	CreatedAt     string              `json:"created_at,omitempty"`
	UpdatedAt     string              `json:"updated_at,omitempty"`
}

// respondError converts a gRPC error into the appropriate HTTP response.
func respondError(c *gin.Context, err error) {
	s, _ := status.FromError(err)
	c.JSON(grpcCodeToHTTP(s.Code()), ErrorResponse{
		Error: s.Message(),
		Code:  grpcCodeToString(s.Code()),
	})
}

func grpcCodeToHTTP(code codes.Code) int {
	switch code {
	case codes.NotFound:
		return http.StatusNotFound
	case codes.AlreadyExists:
		return http.StatusConflict
	case codes.InvalidArgument:
		return http.StatusBadRequest
	case codes.Unauthenticated:
		return http.StatusUnauthorized
	case codes.PermissionDenied:
		return http.StatusForbidden
	case codes.Unavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

func grpcCodeToString(code codes.Code) string {
	switch code {
	case codes.NotFound:
		return "NOT_FOUND"
	case codes.AlreadyExists:
		return "ALREADY_EXISTS"
	case codes.InvalidArgument:
		return "INVALID_INPUT"
	case codes.Unauthenticated:
		return "UNAUTHENTICATED"
	case codes.PermissionDenied:
		return "PERMISSION_DENIED"
	case codes.Unavailable:
		return "UNAVAILABLE"
	default:
		return "INTERNAL"
	}
}

func timestampToString(ts *timestamppb.Timestamp) string {
	if ts == nil {
		return ""
	}
	return ts.AsTime().UTC().Format(time.RFC3339)
}

func userFromProto(u *userv1.User) *UserResponse {
	if u == nil {
		return nil
	}
	return &UserResponse{
		UserID:    u.UserId,
		Email:     u.Email,
		Name:      u.Name,
		Role:      u.Role,
		Status:    u.Status,
		CreatedAt: timestampToString(u.CreatedAt),
		UpdatedAt: timestampToString(u.UpdatedAt),
	}
}

func productFromProto(p *productv1.Product) *ProductResponse {
	if p == nil {
		return nil
	}
	images := p.Images
	if images == nil {
		images = []string{}
	}
	return &ProductResponse{
		ProductID:   p.ProductId,
		SellerID:    p.SellerId,
		CategoryID:  p.CategoryId,
		Name:        p.Name,
		Description: p.Description,
		Price:       p.Price,
		Stock:       p.Stock,
		Status:      p.Status,
		Images:      images,
		CreatedAt:   timestampToString(p.CreatedAt),
		UpdatedAt:   timestampToString(p.UpdatedAt),
	}
}

func productFromSearchItem(item *searchv1.SearchResultItem) *ProductResponse {
	if item == nil {
		return nil
	}
	images := item.Images
	if images == nil {
		images = []string{}
	}
	return &ProductResponse{
		ProductID:   item.ProductId,
		SellerID:    item.SellerId,
		CategoryID:  item.CategoryId,
		Name:        item.Name,
		Description: item.Description,
		Price:       item.Price,
		Images:      images,
	}
}

func cartFromProto(cart *cartv1.Cart) *CartResponse {
	if cart == nil {
		return nil
	}
	items := make([]CartItemResponse, 0, len(cart.Items))
	for _, item := range cart.Items {
		items = append(items, CartItemResponse{
			ProductID:   item.ProductId,
			ProductName: item.ProductName,
			UnitPrice:   item.UnitPrice,
			Quantity:    item.Quantity,
			Subtotal:    item.Subtotal,
		})
	}
	return &CartResponse{
		UserID:     cart.UserId,
		Items:      items,
		TotalPrice: cart.TotalPrice,
		ItemCount:  cart.ItemCount,
	}
}

func orderFromProto(o *orderv1.Order) *OrderResponse {
	if o == nil {
		return nil
	}
	items := make([]OrderItemResponse, 0, len(o.Items))
	for _, item := range o.Items {
		items = append(items, OrderItemResponse{
			ProductID:   item.ProductId,
			ProductName: item.ProductName,
			SellerID:    item.SellerId,
			UnitPrice:   item.UnitPrice,
			Quantity:    item.Quantity,
			Subtotal:    item.Subtotal,
		})
	}
	return &OrderResponse{
		OrderID:       o.OrderId,
		UserID:        o.UserId,
		Status:        o.Status,
		TotalAmount:   o.TotalAmount,
		Currency:      o.Currency,
		Items:         items,
		FailureReason: o.FailureReason,
		CreatedAt:     timestampToString(o.CreatedAt),
		UpdatedAt:     timestampToString(o.UpdatedAt),
	}
}

// contextUserID retrieves the authenticated user ID from Gin context.
func contextUserID(c *gin.Context) string {
	v, _ := c.Get("user_id")
	s, _ := v.(string)
	return s
}

// contextUserRole retrieves the authenticated user role from Gin context.
func contextUserRole(c *gin.Context) string {
	v, _ := c.Get("user_role")
	s, _ := v.(string)
	return s
}
