package upload

import (
	"context"

	"go-starter-kit/modules/primitive"

	"gorm.io/gorm"
)

type RepositoryInterface interface {
	Create(ctx context.Context, file primitive.UploadedFile) (primitive.UploadedFile, error)
	FindByID(ctx context.Context, id string) (primitive.UploadedFile, error)
}

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) RepositoryInterface {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, file primitive.UploadedFile) (primitive.UploadedFile, error) {
	if err := r.db.WithContext(ctx).Create(&file).Error; err != nil {
		return primitive.UploadedFile{}, err
	}
	return file, nil
}

func (r *Repository) FindByID(ctx context.Context, id string) (primitive.UploadedFile, error) {
	var file primitive.UploadedFile
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&file).Error; err != nil {
		return primitive.UploadedFile{}, err
	}
	return file, nil
}
