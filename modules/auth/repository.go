package auth

import (
	"context"
	"errors"

	"go-starter-kit/modules/primitive"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type RepositoryInterface interface {
	FindUserByEmail(ctx context.Context, email string) (primitive.User, error)
	SaveRefreshToken(ctx context.Context, token primitive.RefreshToken) error
	FindRefreshToken(ctx context.Context, tokenHash string) (primitive.RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, tokenHash string) error
	RevokeRefreshTokensByUserID(ctx context.Context, userID uuid.UUID) error
}

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) RepositoryInterface {
	return &Repository{db: db}
}

func (r *Repository) FindUserByEmail(ctx context.Context, email string) (primitive.User, error) {
	var data primitive.User
	if err := r.db.WithContext(ctx).Where("email = ? AND deleted_at IS NULL", email).First(&data).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return primitive.User{}, ErrInvalidCredential
		}
		return primitive.User{}, err
	}
	return data, nil
}

func (r *Repository) SaveRefreshToken(ctx context.Context, token primitive.RefreshToken) error {
	return r.db.WithContext(ctx).Create(&token).Error
}

func (r *Repository) FindRefreshToken(ctx context.Context, tokenHash string) (primitive.RefreshToken, error) {
	var data primitive.RefreshToken
	if err := r.db.WithContext(ctx).
		Where("token_hash = ? AND revoked_at IS NULL AND expires_at > NOW()", tokenHash).
		First(&data).Error; err != nil {
		return primitive.RefreshToken{}, err
	}
	return data, nil
}

func (r *Repository) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	return r.db.WithContext(ctx).
		Model(&primitive.RefreshToken{}).
		Where("token_hash = ? AND revoked_at IS NULL", tokenHash).
		Update("revoked_at", gorm.Expr("NOW()")).Error
}

func (r *Repository) RevokeRefreshTokensByUserID(ctx context.Context, userID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&primitive.RefreshToken{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", gorm.Expr("NOW()")).Error
}
