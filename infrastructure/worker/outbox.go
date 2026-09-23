package worker

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type OutboxEvent struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Topic       string     `gorm:"not null"`
	Payload     string     `gorm:"type:jsonb;not null"`
	Attempts    int        `gorm:"not null;default:0"`
	AvailableAt time.Time  `gorm:"not null;default:now()"`
	ProcessedAt *time.Time `gorm:"index"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (OutboxEvent) TableName() string {
	return "outbox_events"
}

type OutboxRepository struct {
	db *gorm.DB
}

type OutboxHandler func(ctx context.Context, event OutboxEvent) error

func NewOutboxRepository(db *gorm.DB) *OutboxRepository {
	return &OutboxRepository{db: db}
}

func (r *OutboxRepository) Enqueue(ctx context.Context, event OutboxEvent) error {
	return r.db.WithContext(ctx).Create(&event).Error
}

func (r *OutboxRepository) FetchPending(ctx context.Context, limit int) ([]OutboxEvent, error) {
	var events []OutboxEvent
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("processed_at IS NULL AND available_at <= NOW()").
			Order("created_at ASC").
			Limit(limit).
			Find(&events).Error
	})
	return events, err
}

func (r *OutboxRepository) MarkProcessed(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&OutboxEvent{}).Where("id = ?", id).Update("processed_at", gorm.Expr("NOW()")).Error
}

func (r *OutboxRepository) MarkFailed(ctx context.Context, id uuid.UUID, delay time.Duration) error {
	return r.db.WithContext(ctx).Model(&OutboxEvent{}).Where("id = ?", id).Updates(map[string]interface{}{
		"attempts":     gorm.Expr("attempts + 1"),
		"available_at": time.Now().Add(delay),
	}).Error
}

type OutboxWorker struct {
	repository *OutboxRepository
	handlers   map[string]OutboxHandler
	interval   time.Duration
	batchSize  int
}

func NewOutboxWorker(repository *OutboxRepository, handlers map[string]OutboxHandler) *OutboxWorker {
	return &OutboxWorker{repository: repository, handlers: handlers, interval: time.Second, batchSize: 10}
}

func (w *OutboxWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = w.ProcessOnce(ctx)
		}
	}
}

func (w *OutboxWorker) ProcessOnce(ctx context.Context) error {
	events, err := w.repository.FetchPending(ctx, w.batchSize)
	if err != nil {
		return err
	}
	for _, event := range events {
		handler, ok := w.handlers[event.Topic]
		if !ok {
			_ = w.repository.MarkFailed(ctx, event.ID, time.Hour)
			continue
		}
		if err := handler(ctx, event); err != nil {
			if errors.Is(err, context.Canceled) {
				return err
			}
			_ = w.repository.MarkFailed(ctx, event.ID, time.Duration(event.Attempts+1)*time.Minute)
			continue
		}
		_ = w.repository.MarkProcessed(ctx, event.ID)
	}
	return nil
}
