package handler

import (
	"context"

	"github.com/google/uuid"
	apperrors "github.com/yourorg/micromart/pkg/errors"
	userv1 "github.com/yourorg/micromart/proto/user/v1"
	"github.com/yourorg/micromart/services/user/internal/service"
)

type UserHandler struct {
	userv1.UnimplementedUserServiceServer
	userService service.UserService
}

func NewUserHandler(userService service.UserService) *UserHandler {
	return &UserHandler{userService: userService}
}

func (h *UserHandler) Register(ctx context.Context, req *userv1.RegisterRequest) (*userv1.RegisterResponse, error) {
	user, err := h.userService.Register(ctx, req.Email, req.Password, req.Name)
	if err != nil {
		return nil, err
	}
	return &userv1.RegisterResponse{UserId: user.ID.String()}, nil
}

func (h *UserHandler) Login(ctx context.Context, req *userv1.LoginRequest) (*userv1.LoginResponse, error) {
	accessToken, refreshToken, expiresIn, err := h.userService.Login(ctx, req.Email, req.Password)
	if err != nil {
		return nil, err
	}
	return &userv1.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    expiresIn,
	}, nil
}

func (h *UserHandler) Refresh(ctx context.Context, req *userv1.RefreshRequest) (*userv1.RefreshResponse, error) {
	accessToken, newRefreshToken, expiresIn, err := h.userService.Refresh(ctx, req.RefreshToken)
	if err != nil {
		return nil, err
	}
	return &userv1.RefreshResponse{
		AccessToken:  accessToken,
		ExpiresIn:    expiresIn,
		RefreshToken: newRefreshToken,
	}, nil
}

func (h *UserHandler) Logout(ctx context.Context, req *userv1.LogoutRequest) (*userv1.LogoutResponse, error) {
	if err := h.userService.Logout(ctx, req.RefreshToken); err != nil {
		return nil, err
	}
	return &userv1.LogoutResponse{}, nil
}

func (h *UserHandler) GetUser(ctx context.Context, req *userv1.GetUserRequest) (*userv1.GetUserResponse, error) {
	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, apperrors.NewInvalidInput("invalid user_id")
	}
	user, err := h.userService.GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &userv1.GetUserResponse{User: user.ToProto()}, nil
}

func (h *UserHandler) UpdateUser(ctx context.Context, req *userv1.UpdateUserRequest) (*userv1.UpdateUserResponse, error) {
	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, apperrors.NewInvalidInput("invalid user_id")
	}
	user, err := h.userService.UpdateUser(ctx, userID, req.Name)
	if err != nil {
		return nil, err
	}
	return &userv1.UpdateUserResponse{User: user.ToProto()}, nil
}

func (h *UserHandler) GetUserByID(ctx context.Context, req *userv1.GetUserByIDRequest) (*userv1.GetUserByIDResponse, error) {
	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, apperrors.NewInvalidInput("invalid user_id")
	}
	user, err := h.userService.GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &userv1.GetUserByIDResponse{User: user.ToProto()}, nil
}
