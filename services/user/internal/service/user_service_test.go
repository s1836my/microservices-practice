package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apperrors "github.com/yourorg/micromart/pkg/errors"
	"github.com/yourorg/micromart/services/user/internal/model"
	"github.com/yourorg/micromart/services/user/internal/service"
)

// --- mocks ---

type mockUserRepo struct {
	users  map[uuid.UUID]*model.User
	byEmail map[string]*model.User
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{
		users:   make(map[uuid.UUID]*model.User),
		byEmail: make(map[string]*model.User),
	}
}

func (m *mockUserRepo) Create(_ context.Context, user *model.User) (*model.User, error) {
	if _, exists := m.byEmail[user.Email]; exists {
		return nil, apperrors.NewAlreadyExists("email already registered")
	}
	created := &model.User{
		ID:           user.ID,
		Email:        user.Email,
		PasswordHash: user.PasswordHash,
		Name:         user.Name,
		Role:         user.Role,
		Status:       user.Status,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	m.users[created.ID] = created
	m.byEmail[created.Email] = created
	return created, nil
}

func (m *mockUserRepo) FindByID(_ context.Context, id uuid.UUID) (*model.User, error) {
	u, ok := m.users[id]
	if !ok {
		return nil, apperrors.NewNotFound("user not found")
	}
	return u, nil
}

func (m *mockUserRepo) FindByEmail(_ context.Context, email string) (*model.User, error) {
	u, ok := m.byEmail[email]
	if !ok {
		return nil, apperrors.NewNotFound("user not found")
	}
	return u, nil
}

func (m *mockUserRepo) Update(_ context.Context, user *model.User) (*model.User, error) {
	if _, ok := m.users[user.ID]; !ok {
		return nil, apperrors.NewNotFound("user not found")
	}
	updated := &model.User{
		ID:           user.ID,
		Email:        user.Email,
		PasswordHash: user.PasswordHash,
		Name:         user.Name,
		Role:         user.Role,
		Status:       user.Status,
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    time.Now(),
	}
	m.users[updated.ID] = updated
	m.byEmail[updated.Email] = updated
	return updated, nil
}

type mockTokenService struct {
	issuedPairs  int
	revokedTokens []string
}

func (m *mockTokenService) IssueTokenPair(_ context.Context, _ *model.User) (string, string, int64, error) {
	m.issuedPairs++
	return "access-token", "refresh-token", 3600, nil
}

func (m *mockTokenService) IssueAccessToken(_ *model.User) (string, int64, error) {
	return "new-access-token", 3600, nil
}

func (m *mockTokenService) ValidateRefreshToken(_ context.Context, raw string) (uuid.UUID, error) {
	if raw == "valid-refresh-token" || raw == "refresh-token" {
		return uuid.MustParse("00000000-0000-0000-0000-000000000001"), nil
	}
	return uuid.Nil, apperrors.NewUnauthorized("invalid or expired refresh token")
}

func (m *mockTokenService) RevokeRefreshToken(_ context.Context, raw string) error {
	m.revokedTokens = append(m.revokedTokens, raw)
	return nil
}

// --- helpers ---

func setupUserService() (service.UserService, *mockUserRepo, *mockTokenService) {
	userRepo := newMockUserRepo()
	tokenSvc := &mockTokenService{}
	svc := service.NewUserService(userRepo, tokenSvc)
	return svc, userRepo, tokenSvc
}

// --- tests ---

func TestUserService_Register_Success(t *testing.T) {
	svc, _, _ := setupUserService()

	user, err := svc.Register(context.Background(), "alice@example.com", "password123", "Alice")
	require.NoError(t, err)
	assert.Equal(t, "alice@example.com", user.Email)
	assert.Equal(t, "Alice", user.Name)
	assert.Equal(t, model.RoleCustomer, user.Role)
	assert.Equal(t, model.StatusActive, user.Status)
	assert.NotEmpty(t, user.PasswordHash)
	assert.NotEqual(t, "password123", user.PasswordHash)
}

func TestUserService_Register_InvalidEmail(t *testing.T) {
	svc, _, _ := setupUserService()

	_, err := svc.Register(context.Background(), "not-an-email", "password123", "Alice")
	assert.Error(t, err)
	var appErr *apperrors.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeInvalidInput, appErr.Code)
}

func TestUserService_Register_WeakPassword(t *testing.T) {
	svc, _, _ := setupUserService()

	_, err := svc.Register(context.Background(), "alice@example.com", "pass", "Alice")
	assert.Error(t, err)
	var appErr *apperrors.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeInvalidInput, appErr.Code)
}

func TestUserService_Register_DuplicateEmail(t *testing.T) {
	svc, _, _ := setupUserService()

	_, err := svc.Register(context.Background(), "alice@example.com", "password123", "Alice")
	require.NoError(t, err)

	_, err = svc.Register(context.Background(), "alice@example.com", "password456", "Alice2")
	assert.Error(t, err)
	var appErr *apperrors.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeAlreadyExists, appErr.Code)
}

func TestUserService_Login_Success(t *testing.T) {
	svc, _, tokenSvc := setupUserService()

	_, err := svc.Register(context.Background(), "bob@example.com", "password123", "Bob")
	require.NoError(t, err)

	accessToken, refreshToken, expiresIn, err := svc.Login(context.Background(), "bob@example.com", "password123")
	require.NoError(t, err)
	assert.Equal(t, "access-token", accessToken)
	assert.Equal(t, "refresh-token", refreshToken)
	assert.Equal(t, int64(3600), expiresIn)
	assert.Equal(t, 1, tokenSvc.issuedPairs)
}

func TestUserService_Login_WrongPassword(t *testing.T) {
	svc, _, _ := setupUserService()

	_, err := svc.Register(context.Background(), "bob@example.com", "password123", "Bob")
	require.NoError(t, err)

	_, _, _, err = svc.Login(context.Background(), "bob@example.com", "wrongpassword")
	assert.Error(t, err)
	var appErr *apperrors.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeUnauthorized, appErr.Code)
}

func TestUserService_Login_UnknownEmail(t *testing.T) {
	svc, _, _ := setupUserService()

	_, _, _, err := svc.Login(context.Background(), "nobody@example.com", "password123")
	assert.Error(t, err)
	var appErr *apperrors.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeUnauthorized, appErr.Code)
}

func TestUserService_Refresh_ValidToken(t *testing.T) {
	svc, userRepo, _ := setupUserService()

	// Seed user with matching ID
	userID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	userRepo.users[userID] = &model.User{
		ID:     userID,
		Email:  "test@example.com",
		Name:   "Test",
		Role:   model.RoleCustomer,
		Status: model.StatusActive,
	}

	accessToken, newRefresh, expiresIn, err := svc.Refresh(context.Background(), "valid-refresh-token")
	require.NoError(t, err)
	assert.Equal(t, "access-token", accessToken)   // IssueTokenPair returns "access-token"
	assert.Equal(t, "refresh-token", newRefresh)    // IssueTokenPair returns "refresh-token"
	assert.Equal(t, int64(3600), expiresIn)
}

func TestUserService_Refresh_InvalidToken(t *testing.T) {
	svc, _, _ := setupUserService()

	_, _, _, err := svc.Refresh(context.Background(), "invalid-token")
	assert.Error(t, err)
}

func TestUserService_Logout(t *testing.T) {
	svc, _, tokenSvc := setupUserService()

	err := svc.Logout(context.Background(), "some-refresh-token")
	require.NoError(t, err)
	assert.Contains(t, tokenSvc.revokedTokens, "some-refresh-token")
}

func TestUserService_GetUser(t *testing.T) {
	svc, _, _ := setupUserService()

	user, err := svc.Register(context.Background(), "carol@example.com", "password123", "Carol")
	require.NoError(t, err)

	got, err := svc.GetUser(context.Background(), user.ID)
	require.NoError(t, err)
	assert.Equal(t, user.ID, got.ID)
	assert.Equal(t, "carol@example.com", got.Email)
}

func TestUserService_GetUser_NotFound(t *testing.T) {
	svc, _, _ := setupUserService()

	_, err := svc.GetUser(context.Background(), uuid.New())
	assert.Error(t, err)
	var appErr *apperrors.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

func TestUserService_UpdateUser(t *testing.T) {
	svc, _, _ := setupUserService()

	user, err := svc.Register(context.Background(), "dave@example.com", "password123", "Dave")
	require.NoError(t, err)

	updated, err := svc.UpdateUser(context.Background(), user.ID, "David")
	require.NoError(t, err)
	assert.Equal(t, "David", updated.Name)
	assert.Equal(t, user.ID, updated.ID)
	assert.Equal(t, user.Email, updated.Email)
}

func TestUserService_UpdateUser_InvalidName(t *testing.T) {
	svc, _, _ := setupUserService()

	user, err := svc.Register(context.Background(), "eve@example.com", "password123", "Eve")
	require.NoError(t, err)

	_, err = svc.UpdateUser(context.Background(), user.ID, "")
	assert.Error(t, err)
	var appErr *apperrors.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeInvalidInput, appErr.Code)
}
