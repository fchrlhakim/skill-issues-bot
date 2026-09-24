package primitive

const (
	MessageOK                = "ok"
	MessageRegistered        = "registered"
	MessageLoginSuccess      = "login success"
	MessageInvalidBody       = "invalid request body"
	MessageValidationFailed  = "validation failed"
	MessageInvalidCredential = "invalid credentials" // #nosec G101 -- user-facing error message, not a secret.
	MessageUnauthorized      = "unauthorized"
	MessageUserNotFound      = "user not found"
	MessageForbidden         = "forbidden"
	MessageRefreshSuccess    = "refresh success"
	MessageLogoutSuccess     = "logout success"

	DefaultSortCreatedAt = "created_at"
	SortASC              = "ASC"
	SortDESC             = "DESC"
)
