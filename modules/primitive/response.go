package primitive

import (
	"time"

	"github.com/google/uuid"
)

type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at"`
}

type UserResponse struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type UserListResponse struct {
	Items      []UserResponse `json:"items"`
	NextCursor string         `json:"next_cursor,omitempty"`
}

type AuditLogResponse struct {
	ID           uuid.UUID  `json:"id"`
	ActorID      *uuid.UUID `json:"actor_id,omitempty"`
	Action       string     `json:"action"`
	ResourceType string     `json:"resource_type"`
	ResourceID   string     `json:"resource_id,omitempty"`
	Method       string     `json:"method"`
	Path         string     `json:"path"`
	Status       int        `json:"status"`
	IPAddress    string     `json:"ip_address"`
	UserAgent    string     `json:"user_agent"`
	RequestID    string     `json:"request_id"`
	CreatedAt    time.Time  `json:"created_at"`
}

type AuditLogListResponse struct {
	Items      []AuditLogResponse `json:"items"`
	NextCursor string             `json:"next_cursor,omitempty"`
}

func NewAuditLogResponse(log AuditLog) AuditLogResponse {
	return AuditLogResponse{
		ID:           log.ID,
		ActorID:      log.ActorID,
		Action:       log.Action,
		ResourceType: log.ResourceType,
		ResourceID:   log.ResourceID,
		Method:       log.Method,
		Path:         log.Path,
		Status:       log.Status,
		IPAddress:    log.IPAddress,
		UserAgent:    log.UserAgent,
		RequestID:    log.RequestID,
		CreatedAt:    log.CreatedAt,
	}
}

func NewUserResponse(user User) UserResponse {
	return UserResponse{
		ID:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
	}
}

func NewLoginResponse(accessToken, refreshToken string, expiresAt int64) LoginResponse {
	return LoginResponse{AccessToken: accessToken, RefreshToken: refreshToken, ExpiresAt: expiresAt}
}
