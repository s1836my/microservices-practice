package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/yourorg/micromart/services/user/internal/model"
)

type RefreshTokenRepository interface {
	Create(ctx context.Context, token *model.RefreshToken) (*model.RefreshToken, error)
	FindByTokenHash(ctx context.Context, tokenHash string) (*model.RefreshToken, error)
	RevokeByUserID(ctx context.Context, userID uuid.UUID) error
	RevokeByTokenHash(ctx context.Context, tokenHash string) error
}
