package handler_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apperrors "github.com/yourorg/micromart/pkg/errors"
	userv1 "github.com/yourorg/micromart/proto/user/v1"
	"github.com/yourorg/micromart/services/user/internal/handler"
	"github.com/yourorg/micromart/services/user/internal/model"
)

// --- mock ---

type mockUserService struct {
	registerFn  func(ctx context.Context, email, password, name string) (*model.User, error)
	loginFn     func(ctx context.Context, email, password string) (string, string, int64, error)
	refreshFn   func(ctx context.Context, raw string) (string, string, int64, error)
	logoutFn    func(ctx context.Context, raw string) error
	getUserFn   func(ctx context.Context, id uuid.UUID) (*model.User, error)
	updateFn    func(ctx context.Context, id uuid.UUID, name string) (*model.User, error)
}

func (m *mockUserService) Register(ctx context.Context, email, password, name string) (*model.User, error) {
	return m.registerFn(ctx, email, password, name)
}
func (m *mockUserService) Login(ctx context.Context, email, password string) (string, string, int64, error) {
	return m.loginFn(ctx, email, password)
}
func (m *mockUserService) Refresh(ctx context.Context, raw string) (string, string, int64, error) {
	return m.refreshFn(ctx, raw)
}
func (m *mockUserService) Logout(ctx context.Context, raw string) error {
	return m.logoutFn(ctx, raw)
}
func (m *mockUserService) GetUser(ctx context.Context, id uuid.UUID) (*model.User, error) {
	return m.getUserFn(ctx, id)
}
func (m *mockUserService) UpdateUser(ctx context.Context, id uuid.UUID, name string) (*model.User, error) {
	return m.updateFn(ctx, id, name)
}

// --- helpers ---

func sampleUser() *model.User {
	return &model.User{
		ID:     uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"),
		Email:  "user@example.com",
		Name:   "Sample User",
		Role:   model.RoleCustomer,
		Status: model.StatusActive,
	}
}

// --- tests ---

func TestUserHandler_Register(t *testing.T) {
	user := sampleUser()
	svc := &mockUserService{
		registerFn: func(_ context.Context, _, _, _ string) (*model.User, error) {
			return user, nil
		},
	}
	h := handler.NewUserHandler(svc)

	resp, err := h.Register(context.Background(), &userv1.RegisterRequest{
		Email: "user@example.com", Password: "password123", Name: "Sample User",
	})
	require.NoError(t, err)
	assert.Equal(t, user.ID.String(), resp.UserId)
}

func TestUserHandler_Register_ServiceError(t *testing.T) {
	svc := &mockUserService{
		registerFn: func(_ context.Context, _, _, _ string) (*model.User, error) {
			return nil, apperrors.NewAlreadyExists("email already registered")
		},
	}
	h := handler.NewUserHandler(svc)

	_, err := h.Register(context.Background(), &userv1.RegisterRequest{
		Email: "dup@example.com", Password: "password123", Name: "Dup",
	})
	assert.Error(t, err)
}

func TestUserHandler_Login(t *testing.T) {
	svc := &mockUserService{
		loginFn: func(_ context.Context, _, _ string) (string, string, int64, error) {
			return "access", "refresh", 3600, nil
		},
	}
	h := handler.NewUserHandler(svc)

	resp, err := h.Login(context.Background(), &userv1.LoginRequest{
		Email: "user@example.com", Password: "password123",
	})
	require.NoError(t, err)
	assert.Equal(t, "access", resp.AccessToken)
	assert.Equal(t, "refresh", resp.RefreshToken)
	assert.Equal(t, int64(3600), resp.ExpiresIn)
}

func TestUserHandler_Refresh(t *testing.T) {
	svc := &mockUserService{
		refreshFn: func(_ context.Context, _ string) (string, string, int64, error) {
			return "new-access", "new-refresh", 3600, nil
		},
	}
	h := handler.NewUserHandler(svc)

	resp, err := h.Refresh(context.Background(), &userv1.RefreshRequest{RefreshToken: "raw-token"})
	require.NoError(t, err)
	assert.Equal(t, "new-access", resp.AccessToken)
	assert.Equal(t, "new-refresh", resp.RefreshToken)
}

func TestUserHandler_Logout(t *testing.T) {
	called := false
	svc := &mockUserService{
		logoutFn: func(_ context.Context, _ string) error {
			called = true
			return nil
		},
	}
	h := handler.NewUserHandler(svc)

	_, err := h.Logout(context.Background(), &userv1.LogoutRequest{RefreshToken: "raw-token"})
	require.NoError(t, err)
	assert.True(t, called)
}

func TestUserHandler_GetUser(t *testing.T) {
	user := sampleUser()
	svc := &mockUserService{
		getUserFn: func(_ context.Context, id uuid.UUID) (*model.User, error) {
			if id == user.ID {
				return user, nil
			}
			return nil, apperrors.NewNotFound("user not found")
		},
	}
	h := handler.NewUserHandler(svc)

	resp, err := h.GetUser(context.Background(), &userv1.GetUserRequest{UserId: user.ID.String()})
	require.NoError(t, err)
	assert.Equal(t, user.ID.String(), resp.User.UserId)
	assert.Equal(t, user.Email, resp.User.Email)
}

func TestUserHandler_GetUser_InvalidUUID(t *testing.T) {
	svc := &mockUserService{}
	h := handler.NewUserHandler(svc)

	_, err := h.GetUser(context.Background(), &userv1.GetUserRequest{UserId: "not-a-uuid"})
	assert.Error(t, err)
}

func TestUserHandler_UpdateUser(t *testing.T) {
	user := sampleUser()
	svc := &mockUserService{
		updateFn: func(_ context.Context, id uuid.UUID, name string) (*model.User, error) {
			return &model.User{
				ID:    user.ID,
				Email: user.Email,
				Name:  name,
				Role:  user.Role,
			}, nil
		},
	}
	h := handler.NewUserHandler(svc)

	resp, err := h.UpdateUser(context.Background(), &userv1.UpdateUserRequest{
		UserId: user.ID.String(), Name: "Updated Name",
	})
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", resp.User.Name)
}

func TestUserHandler_GetUserByID(t *testing.T) {
	user := sampleUser()
	svc := &mockUserService{
		getUserFn: func(_ context.Context, _ uuid.UUID) (*model.User, error) {
			return user, nil
		},
	}
	h := handler.NewUserHandler(svc)

	resp, err := h.GetUserByID(context.Background(), &userv1.GetUserByIDRequest{UserId: user.ID.String()})
	require.NoError(t, err)
	assert.Equal(t, user.ID.String(), resp.User.UserId)
}
