package user

import (
	"context"
	"errors"
	"time"

	"go-starter-kit/modules/primitive"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type RepositoryInterface interface {
	Create(ctx context.Context, user primitive.User) (primitive.User, error)
	FindByEmail(ctx context.Context, email string) (primitive.User, error)
	FindByID(ctx context.Context, id uuid.UUID) (primitive.User, error)
	FindListKeyset(ctx context.Context, limit int, createdAtCursor *time.Time, idCursor string) ([]primitive.User, error)
}

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) RepositoryInterface {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, data primitive.User) (primitive.User, error) {
	if err := r.db.WithContext(ctx).Create(&data).Error; err != nil {
		return primitive.User{}, err
	}
	return data, nil
}

func (r *Repository) FindByEmail(ctx context.Context, email string) (primitive.User, error) {
	var data primitive.User
	if err := r.db.WithContext(ctx).Where("email = ? AND deleted_at IS NULL", email).First(&data).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return primitive.User{}, ErrNotFound
		}
		return primitive.User{}, err
	}
	return data, nil
}

func (r *Repository) FindByID(ctx context.Context, id uuid.UUID) (primitive.User, error) {
	var data primitive.User
	if err := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&data).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return primitive.User{}, ErrNotFound
		}
		return primitive.User{}, err
	}
	return data, nil
}

func (r *Repository) FindListKeyset(ctx context.Context, limit int, createdAtCursor *time.Time, idCursor string) ([]primitive.User, error) {
	var result []primitive.User
	query := r.db.WithContext(ctx).Where("deleted_at IS NULL")
	if createdAtCursor != nil && idCursor != "" {
		query = query.Where("(created_at, id) < (?, ?)", *createdAtCursor, idCursor)
	}
	if err := query.Order("created_at DESC, id DESC").Limit(limit).Find(&result).Error; err != nil {
		return nil, err
	}
	return result, nil
}

var ErrNotFound = errors.New("user not found")
