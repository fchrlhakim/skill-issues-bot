package health

import (
	"context"

	"gorm.io/gorm"
)

type RepositoryInterface interface {
	PingDatabase(ctx context.Context) error
}

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) RepositoryInterface {
	return &Repository{db: db}
}

func (r *Repository) PingDatabase(ctx context.Context) error {
	db, err := r.db.DB()
	if err != nil {
		return err
	}
	return db.PingContext(ctx)
}
