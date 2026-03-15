package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/timestamppb"

	cartv1 "github.com/yourorg/micromart/proto/cart/v1"
	orderv1 "github.com/yourorg/micromart/proto/order/v1"
	productv1 "github.com/yourorg/micromart/proto/product/v1"
	userv1 "github.com/yourorg/micromart/proto/user/v1"
)

// --- grpcCodeToHTTP ---

func TestGrpcCodeToHTTP(t *testing.T) {
	tests := []struct {
		name     string
		code     codes.Code
		expected int
	}{
		{"NotFound", codes.NotFound, http.StatusNotFound},
		{"AlreadyExists", codes.AlreadyExists, http.StatusConflict},
		{"InvalidArgument", codes.InvalidArgument, http.StatusBadRequest},
		{"Unauthenticated", codes.Unauthenticated, http.StatusUnauthorized},
		{"PermissionDenied", codes.PermissionDenied, http.StatusForbidden},
		{"Unavailable", codes.Unavailable, http.StatusServiceUnavailable},
		{"Internal", codes.Internal, http.StatusInternalServerError},
		{"Unknown", codes.Unknown, http.StatusInternalServerError},
		{"DeadlineExceeded", codes.DeadlineExceeded, http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, grpcCodeToHTTP(tt.code))
		})
	}
}

// --- grpcCodeToString ---

func TestGrpcCodeToString(t *testing.T) {
	tests := []struct {
		name     string
		code     codes.Code
		expected string
	}{
		{"NotFound", codes.NotFound, "NOT_FOUND"},
		{"AlreadyExists", codes.AlreadyExists, "ALREADY_EXISTS"},
		{"InvalidArgument", codes.InvalidArgument, "INVALID_INPUT"},
		{"Unauthenticated", codes.Unauthenticated, "UNAUTHENTICATED"},
		{"PermissionDenied", codes.PermissionDenied, "PERMISSION_DENIED"},
		{"Unavailable", codes.Unavailable, "UNAVAILABLE"},
		{"Internal", codes.Internal, "INTERNAL"},
		{"Unknown", codes.Unknown, "INTERNAL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, grpcCodeToString(tt.code))
		})
	}
}

// --- timestampToString ---

func TestTimestampToString_Nil(t *testing.T) {
	assert.Equal(t, "", timestampToString(nil))
}

func TestTimestampToString_Valid(t *testing.T) {
	ts := timestamppb.New(time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC))
	result := timestampToString(ts)
	assert.Equal(t, "2024-06-15T10:30:00Z", result)
}

// --- userFromProto ---

func TestUserFromProto_Nil(t *testing.T) {
	assert.Nil(t, userFromProto(nil))
}

func TestUserFromProto_Valid(t *testing.T) {
	now := timestamppb.Now()
	u := &userv1.User{
		UserId:    "user-123",
		Email:     "alice@example.com",
		Name:      "Alice",
		Role:      "customer",
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}

	result := userFromProto(u)
	assert.Equal(t, "user-123", result.UserID)
	assert.Equal(t, "alice@example.com", result.Email)
	assert.Equal(t, "Alice", result.Name)
	assert.Equal(t, "customer", result.Role)
	assert.Equal(t, "active", result.Status)
	assert.NotEmpty(t, result.CreatedAt)
	assert.NotEmpty(t, result.UpdatedAt)
}

// --- productFromProto ---

func TestProductFromProto_Nil(t *testing.T) {
	assert.Nil(t, productFromProto(nil))
}

func TestProductFromProto_NilImages_BecomesEmptySlice(t *testing.T) {
	p := &productv1.Product{
		ProductId:   "prod-1",
		SellerId:    "seller-1",
		CategoryId:  "cat-1",
		Name:        "Widget",
		Description: "A fine widget",
		Price:       1000,
		Stock:       50,
		Status:      "active",
		Images:      nil,
	}

	result := productFromProto(p)
	assert.NotNil(t, result)
	assert.Equal(t, "prod-1", result.ProductID)
	assert.Equal(t, "Widget", result.Name)
	assert.Equal(t, int64(1000), result.Price)
	assert.Equal(t, int32(50), result.Stock)
	assert.NotNil(t, result.Images)
	assert.Empty(t, result.Images)
}

func TestProductFromProto_WithImages(t *testing.T) {
	p := &productv1.Product{
		ProductId: "prod-2",
		SellerId:  "seller-2",
		Name:      "Gadget",
		Images:    []string{"img1.jpg", "img2.jpg"},
	}

	result := productFromProto(p)
	assert.Equal(t, []string{"img1.jpg", "img2.jpg"}, result.Images)
}

// --- cartFromProto ---

func TestCartFromProto_Nil(t *testing.T) {
	assert.Nil(t, cartFromProto(nil))
}

func TestCartFromProto_Valid(t *testing.T) {
	cart := &cartv1.Cart{
		UserId: "user-1",
		Items: []*cartv1.CartItem{
			{
				ProductId:   "prod-1",
				ProductName: "Widget",
				UnitPrice:   500,
				Quantity:     2,
				Subtotal:    1000,
			},
			{
				ProductId:   "prod-2",
				ProductName: "Gadget",
				UnitPrice:   1500,
				Quantity:     1,
				Subtotal:    1500,
			},
		},
		TotalPrice: 2500,
		ItemCount:  3,
	}

	result := cartFromProto(cart)
	assert.Equal(t, "user-1", result.UserID)
	assert.Len(t, result.Items, 2)
	assert.Equal(t, "prod-1", result.Items[0].ProductID)
	assert.Equal(t, "Widget", result.Items[0].ProductName)
	assert.Equal(t, int64(500), result.Items[0].UnitPrice)
	assert.Equal(t, int32(2), result.Items[0].Quantity)
	assert.Equal(t, int64(1000), result.Items[0].Subtotal)
	assert.Equal(t, int64(2500), result.TotalPrice)
	assert.Equal(t, int32(3), result.ItemCount)
}

func TestCartFromProto_EmptyItems(t *testing.T) {
	cart := &cartv1.Cart{
		UserId:     "user-1",
		Items:      nil,
		TotalPrice: 0,
		ItemCount:  0,
	}

	result := cartFromProto(cart)
	assert.NotNil(t, result)
	assert.Empty(t, result.Items)
}

// --- orderFromProto ---

func TestOrderFromProto_Nil(t *testing.T) {
	assert.Nil(t, orderFromProto(nil))
}

func TestOrderFromProto_Valid(t *testing.T) {
	now := timestamppb.Now()
	o := &orderv1.Order{
		OrderId:     "order-1",
		UserId:      "user-1",
		Status:      "COMPLETED",
		TotalAmount: 3000,
		Currency:    "JPY",
		Items: []*orderv1.OrderItem{
			{
				ProductId:   "prod-1",
				ProductName: "Widget",
				SellerId:    "seller-1",
				UnitPrice:   1000,
				Quantity:     3,
				Subtotal:    3000,
			},
		},
		FailureReason: "",
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	result := orderFromProto(o)
	assert.Equal(t, "order-1", result.OrderID)
	assert.Equal(t, "user-1", result.UserID)
	assert.Equal(t, "COMPLETED", result.Status)
	assert.Equal(t, int64(3000), result.TotalAmount)
	assert.Equal(t, "JPY", result.Currency)
	assert.Len(t, result.Items, 1)
	assert.Equal(t, "prod-1", result.Items[0].ProductID)
	assert.Equal(t, "seller-1", result.Items[0].SellerID)
	assert.NotEmpty(t, result.CreatedAt)
	assert.Empty(t, result.FailureReason)
}

func TestOrderFromProto_WithFailureReason(t *testing.T) {
	o := &orderv1.Order{
		OrderId:       "order-2",
		UserId:        "user-1",
		Status:        "CANCELLED",
		FailureReason: "payment declined",
	}

	result := orderFromProto(o)
	assert.Equal(t, "CANCELLED", result.Status)
	assert.Equal(t, "payment declined", result.FailureReason)
}

// --- contextUserID / contextUserRole ---

func TestContextUserID_NotSet(t *testing.T) {
	// contextUserID uses c.Get which returns ("", false) if not set
	// We test via the exported behavior in the handler, but this is
	// an internal function. Since we're in the same package, we test directly.
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	assert.Equal(t, "", contextUserID(c))
}

func TestContextUserID_Set(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("user_id", "user-42")
	assert.Equal(t, "user-42", contextUserID(c))
}

func TestContextUserRole_NotSet(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	assert.Equal(t, "", contextUserRole(c))
}

func TestContextUserRole_Set(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("user_role", "admin")
	assert.Equal(t, "admin", contextUserRole(c))
}
