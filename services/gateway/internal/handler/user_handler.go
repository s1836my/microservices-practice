package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/yourorg/micromart/services/gateway/internal/client"
	userv1 "github.com/yourorg/micromart/proto/user/v1"
)

// GetMe handles GET /api/v1/users/me
func (h *Handlers) GetMe(c *gin.Context) {
	userID := contextUserID(c)

	resp, err := client.Execute(h.clients.UserCB, func() (*userv1.GetUserByIDResponse, error) {
		return h.clients.User.GetUserByID(c.Request.Context(), &userv1.GetUserByIDRequest{
			UserId: userID,
		})
	})
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": userFromProto(resp.User)})
}

// UpdateMe handles PUT /api/v1/users/me
func (h *Handlers) UpdateMe(c *gin.Context) {
	userID := contextUserID(c)

	var req struct {
		Name string `json:"name" binding:"required,min=1,max=100"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error(), Code: "INVALID_INPUT"})
		return
	}

	resp, err := client.Execute(h.clients.UserCB, func() (*userv1.UpdateUserResponse, error) {
		return h.clients.User.UpdateUser(c.Request.Context(), &userv1.UpdateUserRequest{
			UserId: userID,
			Name:   req.Name,
		})
	})
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": userFromProto(resp.User)})
}
