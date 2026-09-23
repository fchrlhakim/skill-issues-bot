package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"go-starter-kit/infrastructure/jwt"
	"go-starter-kit/modules/primitive"
	"go-starter-kit/modules/user"
	"go-starter-kit/utils"

	"github.com/google/uuid"
)

type stubAuthRepository struct {
	user              primitive.User
	findUserErr       error
	refreshTokens     map[string]primitive.RefreshToken
	savedTokens       []primitive.RefreshToken
	revokedTokenHash  string
	revokedUserID     uuid.UUID
	revokeTokenCalled bool
	revokeAllCalled   bool
}

func (r *stubAuthRepository) FindUserByEmail(ctx context.Context, email string) (primitive.User, error) {
	if r.findUserErr != nil {
		return primitive.User{}, r.findUserErr
	}
	if r.user.Email != email {
		return primitive.User{}, ErrInvalidCredential
	}
	return r.user, nil
}

func (r *stubAuthRepository) SaveRefreshToken(ctx context.Context, token primitive.RefreshToken) error {
	r.savedTokens = append(r.savedTokens, token)
	if r.refreshTokens == nil {
		r.refreshTokens = map[string]primitive.RefreshToken{}
	}
	r.refreshTokens[token.TokenHash] = token
	return nil
}

func (r *stubAuthRepository) FindRefreshToken(ctx context.Context, tokenHash string) (primitive.RefreshToken, error) {
	token, ok := r.refreshTokens[tokenHash]
	if !ok {
		return primitive.RefreshToken{}, errors.New("token not found")
	}
	return token, nil
}

func (r *stubAuthRepository) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	r.revokeTokenCalled = true
	r.revokedTokenHash = tokenHash
	return nil
}

func (r *stubAuthRepository) RevokeRefreshTokensByUserID(ctx context.Context, userID uuid.UUID) error {
	r.revokeAllCalled = true
	r.revokedUserID = userID
	return nil
}

type stubUserService struct{}

func (s stubUserService) Register(ctx context.Context, input primitive.RegisterUserInput) (primitive.User, error) {
	return primitive.User{Name: input.Name, Email: input.Email}, nil
}

func (s stubUserService) GetProfile(ctx context.Context, id uuid.UUID) (primitive.User, error) {
	return primitive.User{}, nil
}

func (s stubUserService) Authenticate(ctx context.Context, email, password string) (primitive.User, error) {
	return primitive.User{}, nil
}

func (s stubUserService) ListUsers(ctx context.Context, limit int, createdAtCursor *time.Time, idCursor string) (primitive.UserListResponse, error) {
	return primitive.UserListResponse{}, nil
}

var _ RepositoryInterface = (*stubAuthRepository)(nil)
var _ user.ServiceInterface = stubUserService{}

func TestLoginRejectsWrongPassword(t *testing.T) {
	passwordHash, err := utils.HashPassword("correct-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	repository := &stubAuthRepository{user: primitive.User{ID: uuid.New(), Email: "user@example.com", PasswordHash: passwordHash}}
	service := NewService(repository, stubUserService{}, jwt.NewService("test-secret", time.Minute, time.Hour))

	_, err = service.Login(context.Background(), primitive.LoginRequest{Email: "user@example.com", Password: "wrong-password"})

	if !errors.Is(err, ErrInvalidCredential) {
		t.Fatalf("expected ErrInvalidCredential, got %v", err)
	}
	if len(repository.savedTokens) != 0 {
		t.Fatalf("expected no refresh token to be saved, got %d", len(repository.savedTokens))
	}
}

func TestRefreshRevokesOldTokenAndSavesReplacement(t *testing.T) {
	tokenService := jwt.NewService("test-secret", time.Minute, time.Hour)
	userID := uuid.New()
	tokens, err := tokenService.GeneratePair(userID)
	if err != nil {
		t.Fatalf("generate pair: %v", err)
	}
	oldHash := hashToken(tokens.RefreshToken)
	repository := &stubAuthRepository{refreshTokens: map[string]primitive.RefreshToken{
		oldHash: {UserID: userID, TokenHash: oldHash, ExpiresAt: time.Now().Add(time.Hour)},
	}}
	service := NewService(repository, stubUserService{}, tokenService)

	response, err := service.Refresh(context.Background(), primitive.RefreshTokenRequest{RefreshToken: tokens.RefreshToken})

	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if !repository.revokeTokenCalled || repository.revokedTokenHash != oldHash {
		t.Fatalf("expected old refresh token to be revoked")
	}
	if len(repository.savedTokens) != 1 {
		t.Fatalf("expected replacement refresh token to be saved, got %d", len(repository.savedTokens))
	}
	if repository.savedTokens[0].TokenHash == oldHash {
		t.Fatalf("expected replacement token hash to differ from old token hash")
	}
	if response.RefreshToken == "" || response.AccessToken == "" {
		t.Fatalf("expected non-empty token response")
	}
}

func TestRefreshSavesReplacementWithConfiguredRefreshTTL(t *testing.T) {
	refreshTTL := 2 * time.Hour
	tokenService := jwt.NewService("test-secret", time.Minute, refreshTTL)
	userID := uuid.New()
	tokens, err := tokenService.GeneratePair(userID)
	if err != nil {
		t.Fatalf("generate pair: %v", err)
	}
	oldHash := hashToken(tokens.RefreshToken)
	repository := &stubAuthRepository{refreshTokens: map[string]primitive.RefreshToken{
		oldHash: {UserID: userID, TokenHash: oldHash, ExpiresAt: time.Now().Add(refreshTTL)},
	}}
	service := NewService(repository, stubUserService{}, tokenService)
	beforeRefresh := time.Now()

	_, err = service.Refresh(context.Background(), primitive.RefreshTokenRequest{RefreshToken: tokens.RefreshToken})

	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if len(repository.savedTokens) != 1 {
		t.Fatalf("expected replacement refresh token to be saved, got %d", len(repository.savedTokens))
	}
	expiresAt := repository.savedTokens[0].ExpiresAt
	minimum := beforeRefresh.Add(refreshTTL).Add(-5 * time.Second)
	maximum := beforeRefresh.Add(refreshTTL).Add(5 * time.Second)
	if expiresAt.Before(minimum) || expiresAt.After(maximum) {
		t.Fatalf("expected expiry around configured refresh TTL, got %s", expiresAt)
	}
}

func TestLogoutRevokeAllUsesTokenUserID(t *testing.T) {
	tokenService := jwt.NewService("test-secret", time.Minute, time.Hour)
	userID := uuid.New()
	tokens, err := tokenService.GeneratePair(userID)
	if err != nil {
		t.Fatalf("generate pair: %v", err)
	}
	repository := &stubAuthRepository{}
	service := NewService(repository, stubUserService{}, tokenService)

	if err := service.Logout(context.Background(), primitive.LogoutRequest{RefreshToken: tokens.RefreshToken, RevokeAll: true}); err != nil {
		t.Fatalf("logout: %v", err)
	}

	if !repository.revokeAllCalled {
		t.Fatalf("expected revoke all to be called")
	}
	if repository.revokedUserID != userID {
		t.Fatalf("expected revoke all user ID %s, got %s", userID, repository.revokedUserID)
	}
	if repository.revokeTokenCalled {
		t.Fatalf("expected single-token revoke not to be called")
	}
}
