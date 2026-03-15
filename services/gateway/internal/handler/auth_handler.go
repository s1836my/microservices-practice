package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/yourorg/micromart/services/gateway/internal/client"
	userv1 "github.com/yourorg/micromart/proto/user/v1"
)

// Register handles POST /api/v1/auth/register
func (h *Handlers) Register(c *gin.Context) {
	var req struct {
		Email    string `json:"email"    binding:"required,email"`
		Password string `json:"password" binding:"required,min=8"`
		Name     string `json:"name"     binding:"required,min=1,max=100"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: "INVALID_INPUT"})
		return
	}

	resp, err := client.Execute(h.clients.UserCB, func() (*userv1.RegisterResponse, error) {
		return h.clients.User.Register(c.Request.Context(), &userv1.RegisterRequest{
			Email:    req.Email,
			Password: req.Password,
			Name:     req.Name,
		})
	})
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"user_id": resp.UserId})
}

// Login handles POST /api/v1/auth/login
func (h *Handlers) Login(c *gin.Context) {
	var req struct {
		Email    string `json:"email"    binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: "INVALID_INPUT"})
		return
	}

	resp, err := client.Execute(h.clients.UserCB, func() (*userv1.LoginResponse, error) {
		return h.clients.User.Login(c.Request.Context(), &userv1.LoginRequest{
			Email:    req.Email,
			Password: req.Password,
		})
	})
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token":  resp.AccessToken,
		"refresh_token": resp.RefreshToken,
		"expires_in":    resp.ExpiresIn,
	})
}

// Refresh handles POST /api/v1/auth/refresh
func (h *Handlers) Refresh(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: "INVALID_INPUT"})
		return
	}

	resp, err := client.Execute(h.clients.UserCB, func() (*userv1.RefreshResponse, error) {
		return h.clients.User.Refresh(c.Request.Context(), &userv1.RefreshRequest{
			RefreshToken: req.RefreshToken,
		})
	})
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token":  resp.AccessToken,
		"refresh_token": resp.RefreshToken,
		"expires_in":    resp.ExpiresIn,
	})
}
