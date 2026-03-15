package service

import (
	"context"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	apperrors "github.com/yourorg/micromart/pkg/errors"
	"github.com/yourorg/micromart/services/user/internal/model"
	"github.com/yourorg/micromart/services/user/internal/repository"
	"github.com/yourorg/micromart/services/user/internal/validator"
)

// dummyHash はタイミング攻撃対策のためのダミーbcryptハッシュ。
// ユーザーが存在しない場合でもbcrypt比較を実行して応答時間を均一化する。
var dummyHash = func() string {
	h, _ := bcrypt.GenerateFromPassword([]byte("dummy-password-for-timing"), bcryptCost)
	return string(h)
}()

type UserService interface {
	Register(ctx context.Context, email, password, name string) (*model.User, error)
	Login(ctx context.Context, email, password string) (accessToken, refreshToken string, expiresIn int64, err error)
	// Refresh はToken Rotationを実装: 旧トークンを無効化し新しいトークンペアを返す
	Refresh(ctx context.Context, rawRefreshToken string) (accessToken, newRefreshToken string, expiresIn int64, err error)
	Logout(ctx context.Context, rawRefreshToken string) error
	GetUser(ctx context.Context, userID uuid.UUID) (*model.User, error)
	UpdateUser(ctx context.Context, userID uuid.UUID, name string) (*model.User, error)
}

type userService struct {
	userRepo     repository.UserRepository
	tokenService TokenService
}

func NewUserService(userRepo repository.UserRepository, tokenService TokenService) UserService {
	return &userService{
		userRepo:     userRepo,
		tokenService: tokenService,
	}
}

func (s *userService) Register(ctx context.Context, email, password, name string) (*model.User, error) {
	if err := validator.ValidateEmail(email); err != nil {
		return nil, err
	}
	if err := validator.ValidatePassword(password); err != nil {
		return nil, err
	}
	if err := validator.ValidateName(name); err != nil {
		return nil, err
	}

	hash, err := hashPassword(password)
	if err != nil {
		return nil, err
	}

	newUser := &model.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: hash,
		Name:         name,
		Role:         model.RoleCustomer,
		Status:       model.StatusActive,
	}
	created, err := s.userRepo.Create(ctx, newUser)
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (s *userService) Login(ctx context.Context, email, password string) (string, string, int64, error) {
	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		// タイミング攻撃対策: ユーザーが存在しない場合もbcrypt比較を実行して応答時間を均一化
		_ = bcrypt.CompareHashAndPassword([]byte(dummyHash), []byte(password))
		return "", "", 0, apperrors.NewUnauthorized("invalid credentials")
	}

	// ステータス確認はパスワード検証前に行い、アカウント状態を漏らさない
	if user.Status != model.StatusActive {
		_ = bcrypt.CompareHashAndPassword([]byte(dummyHash), []byte(password))
		return "", "", 0, apperrors.NewUnauthorized("invalid credentials")
	}

	if err := verifyPassword(user.PasswordHash, password); err != nil {
		return "", "", 0, err
	}

	accessToken, refreshToken, expiresIn, err := s.tokenService.IssueTokenPair(ctx, user)
	if err != nil {
		return "", "", 0, err
	}
	return accessToken, refreshToken, expiresIn, nil
}

func (s *userService) Refresh(ctx context.Context, rawRefreshToken string) (string, string, int64, error) {
	userID, err := s.tokenService.ValidateRefreshToken(ctx, rawRefreshToken)
	if err != nil {
		return "", "", 0, err
	}

	// Token Rotation: 旧リフレッシュトークンを無効化して漏洩時の被害を最小化
	if err := s.tokenService.RevokeRefreshToken(ctx, rawRefreshToken); err != nil {
		return "", "", 0, err
	}

	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return "", "", 0, err
	}

	return s.tokenService.IssueTokenPair(ctx, user)
}

func (s *userService) Logout(ctx context.Context, rawRefreshToken string) error {
	return s.tokenService.RevokeRefreshToken(ctx, rawRefreshToken)
}

func (s *userService) GetUser(ctx context.Context, userID uuid.UUID) (*model.User, error) {
	return s.userRepo.FindByID(ctx, userID)
}

func (s *userService) UpdateUser(ctx context.Context, userID uuid.UUID, name string) (*model.User, error) {
	if err := validator.ValidateName(name); err != nil {
		return nil, err
	}

	existing, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	toUpdate := &model.User{
		ID:           existing.ID,
		Email:        existing.Email,
		PasswordHash: existing.PasswordHash,
		Name:         name,
		Role:         existing.Role,
		Status:       existing.Status,
		CreatedAt:    existing.CreatedAt,
	}

	updated, err := s.userRepo.Update(ctx, toUpdate)
	if err != nil {
		return nil, err
	}
	return updated, nil
}
