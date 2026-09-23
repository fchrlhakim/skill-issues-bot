package user

import (
	"context"
	"errors"
	"time"

	"go-starter-kit/infrastructure/httplib"
	"go-starter-kit/modules/primitive"
	"go-starter-kit/utils"

	"github.com/google/uuid"
)

type ServiceInterface interface {
	Register(ctx context.Context, input primitive.RegisterUserInput) (primitive.User, error)
	GetProfile(ctx context.Context, id uuid.UUID) (primitive.User, error)
	Authenticate(ctx context.Context, email, password string) (primitive.User, error)
	ListUsers(ctx context.Context, limit int, createdAtCursor *time.Time, idCursor string) (primitive.UserListResponse, error)
}

type Service struct {
	repository RepositoryInterface
}

func NewService(repository RepositoryInterface) ServiceInterface {
	return &Service{repository: repository}
}

func (s *Service) Register(ctx context.Context, input primitive.RegisterUserInput) (primitive.User, error) {
	hashed, err := utils.HashPassword(input.Password)
	if err != nil {
		return primitive.User{}, err
	}
	return s.repository.Create(ctx, primitive.User{Name: input.Name, Email: input.Email, PasswordHash: hashed})
}

func (s *Service) GetProfile(ctx context.Context, id uuid.UUID) (primitive.User, error) {
	return s.repository.FindByID(ctx, id)
}

func (s *Service) Authenticate(ctx context.Context, email, password string) (primitive.User, error) {
	data, err := s.repository.FindByEmail(ctx, email)
	if err != nil {
		return primitive.User{}, errors.New("invalid credentials")
	}
	if !utils.CheckPassword(data.PasswordHash, password) {
		return primitive.User{}, errors.New("invalid credentials")
	}
	return data, nil
}

func (s *Service) ListUsers(ctx context.Context, limit int, createdAtCursor *time.Time, idCursor string) (primitive.UserListResponse, error) {
	users, err := s.repository.FindListKeyset(ctx, limit+1, createdAtCursor, idCursor)
	if err != nil {
		return primitive.UserListResponse{}, err
	}

	hasNext := len(users) > limit
	if hasNext {
		users = users[:limit]
	}

	items := make([]primitive.UserResponse, 0, len(users))
	for _, item := range users {
		items = append(items, primitive.NewUserResponse(item))
	}

	var nextCursor string
	if hasNext && len(users) > 0 {
		last := users[len(users)-1]
		nextCursor = httplib.EncodeCursor(last.CreatedAt, last.ID.String())
	}
	return primitive.UserListResponse{Items: items, NextCursor: nextCursor}, nil
}
