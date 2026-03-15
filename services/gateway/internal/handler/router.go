package handler

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/yourorg/micromart/services/gateway/internal/client"
	"github.com/yourorg/micromart/services/gateway/internal/middleware"
)

// Handlers holds all route handler methods.
type Handlers struct {
	clients *client.Clients
}

// NewHandlers creates a Handlers instance backed by the provided clients.
func NewHandlers(clients *client.Clients) *Handlers {
	return &Handlers{clients: clients}
}

// NewRouter configures and returns the Gin engine.
func NewRouter(h *Handlers, jwtSecret string, rps float64, burst int, log *slog.Logger) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.Logger(log))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "version": "1.0.0"})
	})

	v1 := r.Group("/api/v1")
	v1.Use(middleware.RateLimit(rps, burst))

	// Public: auth
	auth := v1.Group("/auth")
	{
		auth.POST("/register", h.Register)
		auth.POST("/login", h.Login)
		auth.POST("/refresh", h.Refresh)
	}

	// Public: products (read-only)
	v1.GET("/products", h.ListProducts)
	v1.GET("/products/search", h.SearchProducts) // must be before /:product_id
	v1.GET("/products/:product_id", h.GetProduct)

	// Protected: profile
	users := v1.Group("/users")
	users.Use(middleware.Auth(jwtSecret))
	{
		users.GET("/me", h.GetMe)
		users.PUT("/me", h.UpdateMe)
	}

	// Protected: product write
	protectedProducts := v1.Group("/products")
	protectedProducts.Use(middleware.Auth(jwtSecret))
	{
		protectedProducts.POST("", h.CreateProduct)
	}

	// Protected: cart
	cart := v1.Group("/cart")
	cart.Use(middleware.Auth(jwtSecret))
	{
		cart.GET("", h.GetCart)
		cart.POST("/items", h.AddCartItem)
		cart.DELETE("/items/:product_id", h.RemoveCartItem)
	}

	// Protected: orders
	orders := v1.Group("/orders")
	orders.Use(middleware.Auth(jwtSecret))
	{
		orders.POST("", h.CreateOrder)
		orders.GET("", h.ListOrders)
		orders.GET("/:order_id", h.GetOrder)
	}

	return r
}
