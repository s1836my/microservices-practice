package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	apperrors "github.com/yourorg/micromart/pkg/errors"
	"github.com/yourorg/micromart/services/user/internal/model"
)

const pgUniqueViolation = "23505"

type pgUserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) UserRepository {
	return &pgUserRepository{pool: pool}
}

func (r *pgUserRepository) Create(ctx context.Context, user *model.User) (*model.User, error) {
	const q = `
		INSERT INTO users (id, email, password_hash, name, role, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING created_at, updated_at
	`
	var createdAt, updatedAt time.Time
	err := r.pool.QueryRow(ctx, q,
		user.ID, user.Email, user.PasswordHash, user.Name, user.Role, user.Status,
	).Scan(&createdAt, &updatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return nil, apperrors.NewAlreadyExists("email already registered")
		}
		return nil, apperrors.Wrap(apperrors.CodeInternal, err, "create user")
	}
	return &model.User{
		ID:           user.ID,
		Email:        user.Email,
		PasswordHash: user.PasswordHash,
		Name:         user.Name,
		Role:         user.Role,
		Status:       user.Status,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}, nil
}

func (r *pgUserRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	const q = `
		SELECT id, email, password_hash, name, role, status, created_at, updated_at
		FROM users
		WHERE id = $1
	`
	user := &model.User{}
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.Name,
		&user.Role, &user.Status, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NewNotFound("user not found")
		}
		return nil, apperrors.Wrap(apperrors.CodeInternal, err, "find user by id")
	}
	return user, nil
}

func (r *pgUserRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	const q = `
		SELECT id, email, password_hash, name, role, status, created_at, updated_at
		FROM users
		WHERE email = $1
	`
	user := &model.User{}
	err := r.pool.QueryRow(ctx, q, email).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.Name,
		&user.Role, &user.Status, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NewNotFound("user not found")
		}
		return nil, apperrors.Wrap(apperrors.CodeInternal, err, "find user by email")
	}
	return user, nil
}

func (r *pgUserRepository) Update(ctx context.Context, user *model.User) (*model.User, error) {
	const q = `
		UPDATE users
		SET name = $2, updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`
	var updatedAt time.Time
	err := r.pool.QueryRow(ctx, q, user.ID, user.Name).Scan(&updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NewNotFound("user not found")
		}
		return nil, apperrors.Wrap(apperrors.CodeInternal, err, "update user")
	}
	return &model.User{
		ID:           user.ID,
		Email:        user.Email,
		PasswordHash: user.PasswordHash,
		Name:         user.Name,
		Role:         user.Role,
		Status:       user.Status,
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    updatedAt,
	}, nil
}
