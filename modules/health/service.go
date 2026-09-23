package health

import "context"

type ServiceInterface interface {
	Live(ctx context.Context) map[string]string
	Ready(ctx context.Context) error
}

type Service struct {
	repository RepositoryInterface
}

func NewService(repository RepositoryInterface) ServiceInterface {
	return &Service{repository: repository}
}

func (s *Service) Live(ctx context.Context) map[string]string {
	return map[string]string{"status": "ok"}
}

func (s *Service) Ready(ctx context.Context) error {
	return s.repository.PingDatabase(ctx)
}
