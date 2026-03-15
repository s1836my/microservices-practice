package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	apperrors "github.com/yourorg/micromart/pkg/errors"
	"github.com/yourorg/micromart/services/user/internal/model"
	"github.com/yourorg/micromart/services/user/internal/repository"
)

const jwtIssuer = "micromart-user-service"

// UserClaims はJWTのペイロード。Gateway側のミドルウェアと一致させること。
type UserClaims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

type TokenService interface {
	IssueTokenPair(ctx context.Context, user *model.User) (accessToken, refreshToken string, expiresIn int64, err error)
	IssueAccessToken(user *model.User) (accessToken string, expiresIn int64, err error)
	ValidateRefreshToken(ctx context.Context, rawRefreshToken string) (uuid.UUID, error)
	RevokeRefreshToken(ctx context.Context, rawRefreshToken string) error
}

type tokenService struct {
	jwtSecret        []byte
	accessTTL        time.Duration
	refreshTTL       time.Duration
	refreshTokenRepo repository.RefreshTokenRepository
}

func NewTokenService(
	jwtSecret string,
	accessTTL time.Duration,
	refreshTTL time.Duration,
	refreshTokenRepo repository.RefreshTokenRepository,
) TokenService {
	return &tokenService{
		jwtSecret:        []byte(jwtSecret),
		accessTTL:        accessTTL,
		refreshTTL:       refreshTTL,
		refreshTokenRepo: refreshTokenRepo,
	}
}

func (s *tokenService) IssueTokenPair(ctx context.Context, user *model.User) (string, string, int64, error) {
	accessToken, expiresIn, err := s.IssueAccessToken(user)
	if err != nil {
		return "", "", 0, err
	}

	rawRefreshToken, err := generateRawToken()
	if err != nil {
		return "", "", 0, err
	}

	tokenRecord := &model.RefreshToken{
		ID:        uuid.New(),
		UserID:    user.ID,
		TokenHash: hashToken(rawRefreshToken),
		ExpiresAt: time.Now().Add(s.refreshTTL),
	}
	if _, err := s.refreshTokenRepo.Create(ctx, tokenRecord); err != nil {
		return "", "", 0, err
	}

	return accessToken, rawRefreshToken, expiresIn, nil
}

func (s *tokenService) IssueAccessToken(user *model.User) (string, int64, error) {
	expiresAt := time.Now().Add(s.accessTTL)
	claims := &UserClaims{
		UserID: user.ID.String(),
		Email:  user.Email,
		Role:   string(user.Role),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    jwtIssuer,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", 0, apperrors.Wrap(apperrors.CodeInternal, err, "sign jwt")
	}
	return signed, int64(s.accessTTL.Seconds()), nil
}

func (s *tokenService) ValidateRefreshToken(ctx context.Context, rawRefreshToken string) (uuid.UUID, error) {
	tokenHash := hashToken(rawRefreshToken)
	stored, err := s.refreshTokenRepo.FindByTokenHash(ctx, tokenHash)
	if err != nil {
		return uuid.Nil, err
	}
	if stored.IsExpired() {
		return uuid.Nil, apperrors.NewUnauthorized("refresh token expired")
	}
	return stored.UserID, nil
}

func (s *tokenService) RevokeRefreshToken(ctx context.Context, rawRefreshToken string) error {
	return s.refreshTokenRepo.RevokeByTokenHash(ctx, hashToken(rawRefreshToken))
}

func generateRawToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
