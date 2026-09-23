package user

import (
	"context"
	"testing"
	"time"

	"go-starter-kit/infrastructure/httplib"
	"go-starter-kit/modules/primitive"
	"go-starter-kit/utils"

	"github.com/google/uuid"
)

type stubUserRepository struct {
	createdUser primitive.User
	listLimit   int
	users       []primitive.User
}

func (r *stubUserRepository) Create(ctx context.Context, user primitive.User) (primitive.User, error) {
	r.createdUser = user
	user.ID = uuid.New()
	return user, nil
}

func (r *stubUserRepository) FindByEmail(ctx context.Context, email string) (primitive.User, error) {
	return primitive.User{}, ErrNotFound
}

func (r *stubUserRepository) FindByID(ctx context.Context, id uuid.UUID) (primitive.User, error) {
	return primitive.User{ID: id}, nil
}

func (r *stubUserRepository) FindListKeyset(ctx context.Context, limit int, createdAtCursor *time.Time, idCursor string) ([]primitive.User, error) {
	r.listLimit = limit
	return r.users, nil
}

var _ RepositoryInterface = (*stubUserRepository)(nil)

func TestRegisterHashesPasswordBeforePersistingUser(t *testing.T) {
	repository := &stubUserRepository{}
	service := NewService(repository)

	_, err := service.Register(context.Background(), primitive.RegisterUserInput{Name: "Test User", Email: "user@example.com", Password: "plain-password"})

	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if repository.createdUser.PasswordHash == "plain-password" {
		t.Fatalf("expected password to be hashed before persistence")
	}
	if !utils.CheckPassword(repository.createdUser.PasswordHash, "plain-password") {
		t.Fatalf("expected persisted password hash to verify original password")
	}
}

func TestListUsersReturnsNextCursorWhenMoreRowsExist(t *testing.T) {
	firstTime := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	secondTime := firstTime.Add(-time.Minute)
	thirdTime := firstTime.Add(-2 * time.Minute)
	secondID := uuid.New()
	repository := &stubUserRepository{users: []primitive.User{
		{ID: uuid.New(), Name: "One", Email: "one@example.com", CreatedAt: firstTime},
		{ID: secondID, Name: "Two", Email: "two@example.com", CreatedAt: secondTime},
		{ID: uuid.New(), Name: "Three", Email: "three@example.com", CreatedAt: thirdTime},
	}}
	service := NewService(repository)

	result, err := service.ListUsers(context.Background(), 2, nil, "")

	if err != nil {
		t.Fatalf("list users: %v", err)
	}
	if repository.listLimit != 3 {
		t.Fatalf("expected repository limit 3, got %d", repository.listLimit)
	}
	if len(result.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result.Items))
	}
	expectedCursor := httplib.EncodeCursor(secondTime, secondID.String())
	if result.NextCursor != expectedCursor {
		t.Fatalf("expected next cursor %q, got %q", expectedCursor, result.NextCursor)
	}
}

func TestListUsersOmitsNextCursorWhenNoMoreRowsExist(t *testing.T) {
	repository := &stubUserRepository{users: []primitive.User{
		{ID: uuid.New(), Name: "One", Email: "one@example.com", CreatedAt: time.Now()},
	}}
	service := NewService(repository)

	result, err := service.ListUsers(context.Background(), 2, nil, "")

	if err != nil {
		t.Fatalf("list users: %v", err)
	}
	if result.NextCursor != "" {
		t.Fatalf("expected empty next cursor, got %q", result.NextCursor)
	}
}
