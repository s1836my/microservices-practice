package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sony/gobreaker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/yourorg/micromart/services/gateway/internal/client"
	"github.com/yourorg/micromart/services/gateway/internal/handler"

	userv1 "github.com/yourorg/micromart/proto/user/v1"
)

// --- mock UserServiceClient ---

type mockUserServiceClient struct {
	registerFn func(ctx context.Context, in *userv1.RegisterRequest, opts ...grpc.CallOption) (*userv1.RegisterResponse, error)
	loginFn    func(ctx context.Context, in *userv1.LoginRequest, opts ...grpc.CallOption) (*userv1.LoginResponse, error)
	refreshFn  func(ctx context.Context, in *userv1.RefreshRequest, opts ...grpc.CallOption) (*userv1.RefreshResponse, error)
	logoutFn   func(ctx context.Context, in *userv1.LogoutRequest, opts ...grpc.CallOption) (*userv1.LogoutResponse, error)
	getUserFn  func(ctx context.Context, in *userv1.GetUserRequest, opts ...grpc.CallOption) (*userv1.GetUserResponse, error)
	updateFn   func(ctx context.Context, in *userv1.UpdateUserRequest, opts ...grpc.CallOption) (*userv1.UpdateUserResponse, error)
	getByIDFn  func(ctx context.Context, in *userv1.GetUserByIDRequest, opts ...grpc.CallOption) (*userv1.GetUserByIDResponse, error)
}

func (m *mockUserServiceClient) Register(ctx context.Context, in *userv1.RegisterRequest, opts ...grpc.CallOption) (*userv1.RegisterResponse, error) {
	if m.registerFn != nil {
		return m.registerFn(ctx, in, opts...)
	}
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (m *mockUserServiceClient) Login(ctx context.Context, in *userv1.LoginRequest, opts ...grpc.CallOption) (*userv1.LoginResponse, error) {
	if m.loginFn != nil {
		return m.loginFn(ctx, in, opts...)
	}
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (m *mockUserServiceClient) Refresh(ctx context.Context, in *userv1.RefreshRequest, opts ...grpc.CallOption) (*userv1.RefreshResponse, error) {
	if m.refreshFn != nil {
		return m.refreshFn(ctx, in, opts...)
	}
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (m *mockUserServiceClient) Logout(ctx context.Context, in *userv1.LogoutRequest, opts ...grpc.CallOption) (*userv1.LogoutResponse, error) {
	if m.logoutFn != nil {
		return m.logoutFn(ctx, in, opts...)
	}
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (m *mockUserServiceClient) GetUser(ctx context.Context, in *userv1.GetUserRequest, opts ...grpc.CallOption) (*userv1.GetUserResponse, error) {
	if m.getUserFn != nil {
		return m.getUserFn(ctx, in, opts...)
	}
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (m *mockUserServiceClient) UpdateUser(ctx context.Context, in *userv1.UpdateUserRequest, opts ...grpc.CallOption) (*userv1.UpdateUserResponse, error) {
	if m.updateFn != nil {
		return m.updateFn(ctx, in, opts...)
	}
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

func (m *mockUserServiceClient) GetUserByID(ctx context.Context, in *userv1.GetUserByIDRequest, opts ...grpc.CallOption) (*userv1.GetUserByIDResponse, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, in, opts...)
	}
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

// --- helpers ---

func init() {
	gin.SetMode(gin.TestMode)
}

func newTestCircuitBreaker() *gobreaker.CircuitBreaker {
	return gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name: "test",
	})
}

func newTestClients(userClient userv1.UserServiceClient) *client.Clients {
	return &client.Clients{
		User:   userClient,
		UserCB: newTestCircuitBreaker(),
	}
}

func setupAuthHandlerRouter(clients *client.Clients) *gin.Engine {
	h := handler.NewHandlers(clients)
	r := gin.New()
	auth := r.Group("/api/v1/auth")
	{
		auth.POST("/register", h.Register)
		auth.POST("/login", h.Login)
		auth.POST("/refresh", h.Refresh)
	}
	return r
}

func jsonBody(t *testing.T, v any) *bytes.Buffer {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return bytes.NewBuffer(b)
}

// --- Register tests ---

func TestRegister_ValidBody_Returns201(t *testing.T) {
	mock := &mockUserServiceClient{
		registerFn: func(_ context.Context, in *userv1.RegisterRequest, _ ...grpc.CallOption) (*userv1.RegisterResponse, error) {
			assert.Equal(t, "alice@example.com", in.Email)
			assert.Equal(t, "password123", in.Password)
			assert.Equal(t, "Alice", in.Name)
			return &userv1.RegisterResponse{UserId: "new-user-id"}, nil
		},
	}
	router := setupAuthHandlerRouter(newTestClients(mock))

	body := jsonBody(t, map[string]string{
		"email":    "alice@example.com",
		"password": "password123",
		"name":     "Alice",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "new-user-id", resp["user_id"])
}

func TestRegister_InvalidBody_MissingEmail_Returns400(t *testing.T) {
	mock := &mockUserServiceClient{}
	router := setupAuthHandlerRouter(newTestClients(mock))

	body := jsonBody(t, map[string]string{
		"password": "password123",
		"name":     "Alice",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "INVALID_INPUT", resp["code"])
}

func TestRegister_InvalidBody_ShortPassword_Returns400(t *testing.T) {
	mock := &mockUserServiceClient{}
	router := setupAuthHandlerRouter(newTestClients(mock))

	body := jsonBody(t, map[string]string{
		"email":    "alice@example.com",
		"password": "short",
		"name":     "Alice",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRegister_InvalidBody_BadEmail_Returns400(t *testing.T) {
	mock := &mockUserServiceClient{}
	router := setupAuthHandlerRouter(newTestClients(mock))

	body := jsonBody(t, map[string]string{
		"email":    "not-an-email",
		"password": "password123",
		"name":     "Alice",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRegister_GRPCAlreadyExists_Returns409(t *testing.T) {
	mock := &mockUserServiceClient{
		registerFn: func(_ context.Context, _ *userv1.RegisterRequest, _ ...grpc.CallOption) (*userv1.RegisterResponse, error) {
			return nil, status.Error(codes.AlreadyExists, "email already registered")
		},
	}
	router := setupAuthHandlerRouter(newTestClients(mock))

	body := jsonBody(t, map[string]string{
		"email":    "alice@example.com",
		"password": "password123",
		"name":     "Alice",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
	var resp map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "email already registered", resp["error"])
}

func TestRegister_GRPCInternalError_Returns500(t *testing.T) {
	mock := &mockUserServiceClient{
		registerFn: func(_ context.Context, _ *userv1.RegisterRequest, _ ...grpc.CallOption) (*userv1.RegisterResponse, error) {
			return nil, status.Error(codes.Internal, "database error")
		},
	}
	router := setupAuthHandlerRouter(newTestClients(mock))

	body := jsonBody(t, map[string]string{
		"email":    "alice@example.com",
		"password": "password123",
		"name":     "Alice",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- Login tests ---

func TestLogin_ValidCredentials_Returns200(t *testing.T) {
	mock := &mockUserServiceClient{
		loginFn: func(_ context.Context, in *userv1.LoginRequest, _ ...grpc.CallOption) (*userv1.LoginResponse, error) {
			assert.Equal(t, "bob@example.com", in.Email)
			assert.Equal(t, "password123", in.Password)
			return &userv1.LoginResponse{
				AccessToken:  "access-token-value",
				RefreshToken: "refresh-token-value",
				ExpiresIn:    3600,
			}, nil
		},
	}
	router := setupAuthHandlerRouter(newTestClients(mock))

	body := jsonBody(t, map[string]string{
		"email":    "bob@example.com",
		"password": "password123",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "access-token-value", resp["access_token"])
	assert.Equal(t, "refresh-token-value", resp["refresh_token"])
	assert.Equal(t, float64(3600), resp["expires_in"])
}

func TestLogin_InvalidBody_Returns400(t *testing.T) {
	mock := &mockUserServiceClient{}
	router := setupAuthHandlerRouter(newTestClients(mock))

	body := jsonBody(t, map[string]string{
		"email": "bob@example.com",
		// missing password
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLogin_WrongPassword_Returns401(t *testing.T) {
	mock := &mockUserServiceClient{
		loginFn: func(_ context.Context, _ *userv1.LoginRequest, _ ...grpc.CallOption) (*userv1.LoginResponse, error) {
			return nil, status.Error(codes.Unauthenticated, "invalid credentials")
		},
	}
	router := setupAuthHandlerRouter(newTestClients(mock))

	body := jsonBody(t, map[string]string{
		"email":    "bob@example.com",
		"password": "wrong-password",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	var resp map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "invalid credentials", resp["error"])
}

func TestLogin_ServiceUnavailable_Returns503(t *testing.T) {
	mock := &mockUserServiceClient{
		loginFn: func(_ context.Context, _ *userv1.LoginRequest, _ ...grpc.CallOption) (*userv1.LoginResponse, error) {
			return nil, status.Error(codes.Unavailable, "service unavailable")
		},
	}
	router := setupAuthHandlerRouter(newTestClients(mock))

	body := jsonBody(t, map[string]string{
		"email":    "bob@example.com",
		"password": "password123",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// --- Refresh tests ---

func TestRefresh_ValidToken_Returns200(t *testing.T) {
	mock := &mockUserServiceClient{
		refreshFn: func(_ context.Context, in *userv1.RefreshRequest, _ ...grpc.CallOption) (*userv1.RefreshResponse, error) {
			assert.Equal(t, "old-refresh-token", in.RefreshToken)
			return &userv1.RefreshResponse{
				AccessToken:  "new-access-token",
				RefreshToken: "new-refresh-token",
				ExpiresIn:    7200,
			}, nil
		},
	}
	router := setupAuthHandlerRouter(newTestClients(mock))

	body := jsonBody(t, map[string]string{
		"refresh_token": "old-refresh-token",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/refresh", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "new-access-token", resp["access_token"])
	assert.Equal(t, "new-refresh-token", resp["refresh_token"])
	assert.Equal(t, float64(7200), resp["expires_in"])
}

func TestRefresh_MissingToken_Returns400(t *testing.T) {
	mock := &mockUserServiceClient{}
	router := setupAuthHandlerRouter(newTestClients(mock))

	body := jsonBody(t, map[string]string{})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/refresh", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRefresh_InvalidToken_Returns401(t *testing.T) {
	mock := &mockUserServiceClient{
		refreshFn: func(_ context.Context, _ *userv1.RefreshRequest, _ ...grpc.CallOption) (*userv1.RefreshResponse, error) {
			return nil, status.Error(codes.Unauthenticated, "invalid refresh token")
		},
	}
	router := setupAuthHandlerRouter(newTestClients(mock))

	body := jsonBody(t, map[string]string{
		"refresh_token": "expired-token",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/refresh", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRegister_EmptyBody_Returns400(t *testing.T) {
	mock := &mockUserServiceClient{}
	router := setupAuthHandlerRouter(newTestClients(mock))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(""))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
