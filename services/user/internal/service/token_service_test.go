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

// --- mock ---

type mockRefreshTokenRepo struct {
	tokens map[string]*model.RefreshToken
}

func newMockRefreshTokenRepo() *mockRefreshTokenRepo {
	return &mockRefreshTokenRepo{tokens: make(map[string]*model.RefreshToken)}
}

func (m *mockRefreshTokenRepo) Create(_ context.Context, token *model.RefreshToken) (*model.RefreshToken, error) {
	created := &model.RefreshToken{
		ID:        token.ID,
		UserID:    token.UserID,
		TokenHash: token.TokenHash,
		ExpiresAt: token.ExpiresAt,
		Revoked:   false,
		CreatedAt: time.Now(),
	}
	m.tokens[token.TokenHash] = created
	return created, nil
}

func (m *mockRefreshTokenRepo) FindByTokenHash(_ context.Context, tokenHash string) (*model.RefreshToken, error) {
	t, ok := m.tokens[tokenHash]
	if !ok {
		return nil, apperrors.NewUnauthorized("invalid or expired refresh token")
	}
	if t.Revoked {
		return nil, apperrors.NewUnauthorized("invalid or expired refresh token")
	}
	return t, nil
}

func (m *mockRefreshTokenRepo) RevokeByUserID(_ context.Context, userID uuid.UUID) error {
	for _, t := range m.tokens {
		if t.UserID == userID {
			t.Revoked = true
		}
	}
	return nil
}

func (m *mockRefreshTokenRepo) RevokeByTokenHash(_ context.Context, tokenHash string) error {
	if t, ok := m.tokens[tokenHash]; ok {
		t.Revoked = true
	}
	return nil
}

// --- helpers ---

func testUser() *model.User {
	return &model.User{
		ID:     uuid.New(),
		Email:  "test@example.com",
		Name:   "Test User",
		Role:   model.RoleCustomer,
		Status: model.StatusActive,
	}
}

func newTokenService(repo *mockRefreshTokenRepo) service.TokenService {
	return service.NewTokenService("test-secret-key", time.Hour, 24*time.Hour, repo)
}

// --- tests ---

func TestTokenService_IssueTokenPair(t *testing.T) {
	repo := newMockRefreshTokenRepo()
	svc := newTokenService(repo)
	user := testUser()

	accessToken, refreshToken, expiresIn, err := svc.IssueTokenPair(context.Background(), user)
	require.NoError(t, err)
	assert.NotEmpty(t, accessToken)
	assert.NotEmpty(t, refreshToken)
	assert.Equal(t, int64(3600), expiresIn)
	assert.Len(t, repo.tokens, 1)
}

func TestTokenService_IssueAccessToken(t *testing.T) {
	svc := newTokenService(newMockRefreshTokenRepo())
	user := testUser()

	token, expiresIn, err := svc.IssueAccessToken(user)
	require.NoError(t, err)
	assert.NotEmpty(t, token)
	assert.Equal(t, int64(3600), expiresIn)
}

func TestTokenService_ValidateRefreshToken(t *testing.T) {
	repo := newMockRefreshTokenRepo()
	svc := newTokenService(repo)
	user := testUser()

	_, rawRefresh, _, err := svc.IssueTokenPair(context.Background(), user)
	require.NoError(t, err)

	userID, err := svc.ValidateRefreshToken(context.Background(), rawRefresh)
	require.NoError(t, err)
	assert.Equal(t, user.ID, userID)
}

func TestTokenService_ValidateRefreshToken_Invalid(t *testing.T) {
	svc := newTokenService(newMockRefreshTokenRepo())

	_, err := svc.ValidateRefreshToken(context.Background(), "invalid-token")
	assert.Error(t, err)
	var appErr *apperrors.AppError
	assert.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeUnauthorized, appErr.Code)
}

func TestTokenService_RevokeRefreshToken(t *testing.T) {
	repo := newMockRefreshTokenRepo()
	svc := newTokenService(repo)
	user := testUser()

	_, rawRefresh, _, err := svc.IssueTokenPair(context.Background(), user)
	require.NoError(t, err)

	err = svc.RevokeRefreshToken(context.Background(), rawRefresh)
	require.NoError(t, err)

	_, err = svc.ValidateRefreshToken(context.Background(), rawRefresh)
	assert.Error(t, err)
}
