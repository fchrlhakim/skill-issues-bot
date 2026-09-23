package primitive

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Name         string     `gorm:"not null" json:"name"`
	Email        string     `gorm:"not null;uniqueIndex" json:"email"`
	PasswordHash string     `gorm:"column:password_hash;not null" json:"-"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	DeletedAt    *time.Time `gorm:"index" json:"-"`
}

func (User) TableName() string {
	return "users"
}

type RefreshToken struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID    uuid.UUID  `gorm:"type:uuid;not null;index" json:"user_id"`
	TokenHash string     `gorm:"column:token_hash;not null;uniqueIndex" json:"-"`
	ExpiresAt time.Time  `gorm:"not null;index" json:"expires_at"`
	RevokedAt *time.Time `gorm:"index" json:"revoked_at"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type UploadedFile struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	OwnerID     uuid.UUID `gorm:"type:uuid;not null;index" json:"owner_id"`
	Original    string    `gorm:"not null" json:"original"`
	Path        string    `gorm:"not null" json:"path"`
	Size        int64     `gorm:"not null" json:"size"`
	ContentType string    `gorm:"not null" json:"content_type"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (UploadedFile) TableName() string {
	return "uploaded_files"
}

type AuditLog struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ActorID      *uuid.UUID `gorm:"type:uuid;index" json:"actor_id,omitempty"`
	Action       string     `gorm:"not null;index" json:"action"`
	ResourceType string     `gorm:"not null;index" json:"resource_type"`
	ResourceID   string     `gorm:"index" json:"resource_id,omitempty"`
	Method       string     `gorm:"not null" json:"method"`
	Path         string     `gorm:"not null" json:"path"`
	Status       int        `gorm:"not null" json:"status"`
	IPAddress    string     `gorm:"not null" json:"ip_address"`
	UserAgent    string     `gorm:"not null" json:"user_agent"`
	RequestID    string     `gorm:"not null;index" json:"request_id"`
	Metadata     string     `gorm:"type:jsonb;not null;default:'{}'" json:"metadata"`
	CreatedAt    time.Time  `gorm:"not null;default:now();index" json:"created_at"`
}

func (AuditLog) TableName() string {
	return "audit_logs"
}

func (RefreshToken) TableName() string {
	return "refresh_tokens"
}
