package audit_log

import (
	"context"
	"time"

	"go-starter-kit/infrastructure/httplib"
	"go-starter-kit/modules/primitive"
)

type ServiceInterface interface {
	Record(ctx context.Context, log primitive.AuditLog) error
	List(ctx context.Context, param primitive.ParameterFindAuditLog, createdAtCursor *time.Time) (primitive.AuditLogListResponse, error)
}

type Service struct {
	repository RepositoryInterface
}

func NewService(repository RepositoryInterface) ServiceInterface {
	return &Service{repository: repository}
}

func (s *Service) Record(ctx context.Context, log primitive.AuditLog) error {
	return s.repository.Create(ctx, log)
}

func (s *Service) List(ctx context.Context, param primitive.ParameterFindAuditLog, createdAtCursor *time.Time) (primitive.AuditLogListResponse, error) {
	logs, err := s.repository.FindListKeyset(ctx, param, createdAtCursor)
	if err != nil {
		return primitive.AuditLogListResponse{}, err
	}

	hasNext := len(logs) > param.Limit-1
	if hasNext {
		logs = logs[:param.Limit-1]
	}

	items := make([]primitive.AuditLogResponse, 0, len(logs))
	for _, item := range logs {
		items = append(items, primitive.NewAuditLogResponse(item))
	}

	var nextCursor string
	if hasNext && len(logs) > 0 {
		last := logs[len(logs)-1]
		nextCursor = httplib.EncodeCursor(last.CreatedAt, last.ID.String())
	}
	return primitive.AuditLogListResponse{Items: items, NextCursor: nextCursor}, nil
}
