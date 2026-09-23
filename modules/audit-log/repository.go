package audit_log

import (
	"context"
	"time"

	"go-starter-kit/modules/primitive"

	"gorm.io/gorm"
)

type RepositoryInterface interface {
	Create(ctx context.Context, log primitive.AuditLog) error
	FindListKeyset(ctx context.Context, param primitive.ParameterFindAuditLog, createdAtCursor *time.Time) ([]primitive.AuditLog, error)
}

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) RepositoryInterface {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, log primitive.AuditLog) error {
	return r.db.WithContext(ctx).Create(&log).Error
}

func (r *Repository) FindListKeyset(ctx context.Context, param primitive.ParameterFindAuditLog, createdAtCursor *time.Time) ([]primitive.AuditLog, error) {
	query := r.db.WithContext(ctx).Model(&primitive.AuditLog{})
	if param.ActorID != "" {
		query = query.Where("actor_id = ?", param.ActorID)
	}
	if param.Action != "" {
		query = query.Where("action = ?", param.Action)
	}
	if param.ResourceType != "" {
		query = query.Where("resource_type = ?", param.ResourceType)
	}
	if param.ResourceID != "" {
		query = query.Where("resource_id = ?", param.ResourceID)
	}
	if createdAtCursor != nil && param.ID != "" {
		query = query.Where("(created_at, id) < (?, ?)", *createdAtCursor, param.ID)
	}

	var result []primitive.AuditLog
	if err := query.Order("created_at DESC, id DESC").Limit(param.Limit).Find(&result).Error; err != nil {
		return nil, err
	}
	return result, nil
}
