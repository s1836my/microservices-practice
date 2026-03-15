package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	apperrors "github.com/yourorg/micromart/pkg/errors"
	"github.com/yourorg/micromart/services/user/internal/model"
)

type pgRefreshTokenRepository struct {
	pool *pgxpool.Pool
}

func NewRefreshTokenRepository(pool *pgxpool.Pool) RefreshTokenRepository {
	return &pgRefreshTokenRepository{pool: pool}
}

func (r *pgRefreshTokenRepository) Create(ctx context.Context, token *model.RefreshToken) (*model.RefreshToken, error) {
	const q = `
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING created_at
	`
	var createdAt time.Time
	err := r.pool.QueryRow(ctx, q,
		token.ID, token.UserID, token.TokenHash, token.ExpiresAt,
	).Scan(&createdAt)
	if err != nil {
		return nil, apperrors.Wrap(apperrors.CodeInternal, err, "create refresh token")
	}
	return &model.RefreshToken{
		ID:        token.ID,
		UserID:    token.UserID,
		TokenHash: token.TokenHash,
		ExpiresAt: token.ExpiresAt,
		Revoked:   false,
		CreatedAt: createdAt,
	}, nil
}

func (r *pgRefreshTokenRepository) FindByTokenHash(ctx context.Context, tokenHash string) (*model.RefreshToken, error) {
	const q = `
		SELECT id, user_id, token_hash, expires_at, revoked, created_at
		FROM refresh_tokens
		WHERE token_hash = $1 AND revoked = FALSE AND expires_at > NOW()
	`
	token := &model.RefreshToken{}
	err := r.pool.QueryRow(ctx, q, tokenHash).Scan(
		&token.ID, &token.UserID, &token.TokenHash,
		&token.ExpiresAt, &token.Revoked, &token.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NewUnauthorized("invalid or expired refresh token")
		}
		return nil, apperrors.Wrap(apperrors.CodeInternal, err, "find refresh token")
	}
	return token, nil
}

func (r *pgRefreshTokenRepository) RevokeByUserID(ctx context.Context, userID uuid.UUID) error {
	const q = `UPDATE refresh_tokens SET revoked = TRUE WHERE user_id = $1`
	if _, err := r.pool.Exec(ctx, q, userID); err != nil {
		return apperrors.Wrap(apperrors.CodeInternal, err, "revoke refresh tokens by user id")
	}
	return nil
}

func (r *pgRefreshTokenRepository) RevokeByTokenHash(ctx context.Context, tokenHash string) error {
	const q = `UPDATE refresh_tokens SET revoked = TRUE WHERE token_hash = $1`
	if _, err := r.pool.Exec(ctx, q, tokenHash); err != nil {
		return apperrors.Wrap(apperrors.CodeInternal, err, "revoke refresh token")
	}
	return nil
}
