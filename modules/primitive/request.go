package primitive

type RegisterUserRequest struct {
	Name     string `json:"name" validate:"required"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
	RevokeAll    bool   `json:"revoke_all"`
}

type RegisterUserInput struct {
	Name     string
	Email    string
	Password string
}

type ParameterFindUser struct {
	Name      string
	Email     string
	PageSize  int
	Offset    int
	SortBy    string
	SortOrder string
}

type ParameterFindAuditLog struct {
	ActorID      string
	Action       string
	ResourceType string
	ResourceID   string
	Limit        int
	CreatedAt    string
	ID           string
}
