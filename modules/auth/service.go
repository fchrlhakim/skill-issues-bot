package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"go-starter-kit/infrastructure/jwt"
	"go-starter-kit/modules/primitive"
	"go-starter-kit/modules/user"
	"go-starter-kit/utils"
)

type ServiceInterface interface {
	Register(ctx context.Context, input primitive.RegisterUserInput) (primitive.User, error)
	Login(ctx context.Context, request primitive.LoginRequest) (primitive.LoginResponse, error)
	Refresh(ctx context.Context, request primitive.RefreshTokenRequest) (primitive.LoginResponse, error)
	Logout(ctx context.Context, request primitive.LogoutRequest) error
}

type Service struct {
	repository  RepositoryInterface
	userService user.ServiceInterface
	token       *jwt.Service
}

func NewService(repository RepositoryInterface, userService user.ServiceInterface, token *jwt.Service) ServiceInterface {
	return &Service{repository: repository, userService: userService, token: token}
}

func (s *Service) Register(ctx context.Context, input primitive.RegisterUserInput) (primitive.User, error) {
	return s.userService.Register(ctx, input)
}

func (s *Service) Login(ctx context.Context, request primitive.LoginRequest) (primitive.LoginResponse, error) {
	data, err := s.repository.FindUserByEmail(ctx, request.Email)
	if err != nil {
		return primitive.LoginResponse{}, err
	}
	if !utils.CheckPassword(data.PasswordHash, request.Password) {
		return primitive.LoginResponse{}, ErrInvalidCredential
	}

	tokens, err := s.token.GeneratePair(data.ID)
	if err != nil {
		return primitive.LoginResponse{}, err
	}
	if err := s.repository.SaveRefreshToken(ctx, primitive.RefreshToken{
		UserID:    data.ID,
		TokenHash: hashToken(tokens.RefreshToken),
		ExpiresAt: time.Now().Add(s.token.RefreshTTL()),
	}); err != nil {
		return primitive.LoginResponse{}, err
	}
	return primitive.NewLoginResponse(tokens.AccessToken, tokens.RefreshToken, tokens.ExpiresAt), nil
}

var ErrInvalidCredential = errors.New("invalid credentials")

func (s *Service) Refresh(ctx context.Context, request primitive.RefreshTokenRequest) (primitive.LoginResponse, error) {
	claims, err := s.token.ParseRefreshToken(request.RefreshToken)
	if err != nil {
		return primitive.LoginResponse{}, err
	}
	tokenHash := hashToken(request.RefreshToken)
	if _, err := s.repository.FindRefreshToken(ctx, tokenHash); err != nil {
		return primitive.LoginResponse{}, err
	}
	if err := s.repository.RevokeRefreshToken(ctx, tokenHash); err != nil {
		return primitive.LoginResponse{}, err
	}

	tokens, err := s.token.GeneratePair(claims.UserID)
	if err != nil {
		return primitive.LoginResponse{}, err
	}
	if err := s.repository.SaveRefreshToken(ctx, primitive.RefreshToken{
		UserID:    claims.UserID,
		TokenHash: hashToken(tokens.RefreshToken),
		ExpiresAt: time.Now().Add(s.token.RefreshTTL()),
	}); err != nil {
		return primitive.LoginResponse{}, err
	}
	return primitive.NewLoginResponse(tokens.AccessToken, tokens.RefreshToken, tokens.ExpiresAt), nil
}

func (s *Service) Logout(ctx context.Context, request primitive.LogoutRequest) error {
	claims, err := s.token.ParseRefreshToken(request.RefreshToken)
	if err != nil {
		return err
	}
	if request.RevokeAll {
		return s.repository.RevokeRefreshTokensByUserID(ctx, claims.UserID)
	}
	return s.repository.RevokeRefreshToken(ctx, hashToken(request.RefreshToken))
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
