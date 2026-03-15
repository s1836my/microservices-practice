package client

import (
	"fmt"
	"time"

	"github.com/sony/gobreaker"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	cartv1 "github.com/yourorg/micromart/proto/cart/v1"
	orderv1 "github.com/yourorg/micromart/proto/order/v1"
	productv1 "github.com/yourorg/micromart/proto/product/v1"
	searchv1 "github.com/yourorg/micromart/proto/search/v1"
	userv1 "github.com/yourorg/micromart/proto/user/v1"
)

// Clients holds gRPC clients and circuit breakers for all backend services.
type Clients struct {
	User    userv1.UserServiceClient
	Product productv1.ProductServiceClient
	Search  searchv1.SearchServiceClient
	Cart    cartv1.CartServiceClient
	Order   orderv1.OrderServiceClient

	UserCB    *gobreaker.CircuitBreaker
	ProductCB *gobreaker.CircuitBreaker
	SearchCB  *gobreaker.CircuitBreaker
	CartCB    *gobreaker.CircuitBreaker
	OrderCB   *gobreaker.CircuitBreaker

	conns []*grpc.ClientConn
}

// Close releases all underlying gRPC connections.
func (c *Clients) Close() {
	for _, conn := range c.conns {
		conn.Close()
	}
}

// New creates gRPC clients for all backend services.
// Connections are established lazily; this call does not fail if services are unavailable.
func New(userAddr, productAddr, searchAddr, cartAddr, orderAddr string) (*Clients, error) {
	userConn, err := newConn(userAddr)
	if err != nil {
		return nil, fmt.Errorf("connect to user service: %w", err)
	}

	productConn, err := newConn(productAddr)
	if err != nil {
		userConn.Close()
		return nil, fmt.Errorf("connect to product service: %w", err)
	}

	searchConn, err := newConn(searchAddr)
	if err != nil {
		userConn.Close()
		productConn.Close()
		return nil, fmt.Errorf("connect to search service: %w", err)
	}

	cartConn, err := newConn(cartAddr)
	if err != nil {
		userConn.Close()
		productConn.Close()
		searchConn.Close()
		return nil, fmt.Errorf("connect to cart service: %w", err)
	}

	orderConn, err := newConn(orderAddr)
	if err != nil {
		userConn.Close()
		productConn.Close()
		searchConn.Close()
		cartConn.Close()
		return nil, fmt.Errorf("connect to order service: %w", err)
	}

	return &Clients{
		User:    userv1.NewUserServiceClient(userConn),
		Product: productv1.NewProductServiceClient(productConn),
		Search:  searchv1.NewSearchServiceClient(searchConn),
		Cart:    cartv1.NewCartServiceClient(cartConn),
		Order:   orderv1.NewOrderServiceClient(orderConn),

		UserCB:    newCircuitBreaker("user-service"),
		ProductCB: newCircuitBreaker("product-service"),
		SearchCB:  newCircuitBreaker("search-service"),
		CartCB:    newCircuitBreaker("cart-service"),
		OrderCB:   newCircuitBreaker("order-service"),

		conns: []*grpc.ClientConn{userConn, productConn, searchConn, cartConn, orderConn},
	}, nil
}

// Execute runs fn through cb. Handles the any→T type assertion.
func Execute[T any](cb *gobreaker.CircuitBreaker, fn func() (T, error)) (T, error) {
	result, err := cb.Execute(func() (any, error) {
		return fn()
	})
	if err != nil {
		var zero T
		return zero, err
	}
	return result.(T), nil
}

func newConn(addr string) (*grpc.ClientConn, error) {
	return grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
}

func newCircuitBreaker(name string) *gobreaker.CircuitBreaker {
	return gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        name,
		MaxRequests: 5,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.Requests >= 3 &&
				float64(counts.TotalFailures)/float64(counts.Requests) >= 0.5
		},
	})
}
