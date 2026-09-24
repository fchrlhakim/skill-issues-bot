package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// The operator gate guards the user directory and the audit trail. It must fail
// closed: an authenticated caller with no allowlist entry — or a deployment that
// never configured one — gets nothing. It used to be wide open to any JWT.
func TestIsOperatorFailsClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	caller := uuid.New()

	cases := []struct {
		name      string
		setup     func(c *gin.Context)
		wantAllow bool
	}{
		{
			name:      "no allowlist in context",
			setup:     func(c *gin.Context) { c.Set(UserIDKey, caller) },
			wantAllow: false,
		},
		{
			name: "empty allowlist",
			setup: func(c *gin.Context) {
				c.Set(UserIDKey, caller)
				c.Set(OperatorAllowlistKey, map[uuid.UUID]struct{}{})
			},
			wantAllow: false,
		},
		{
			name: "caller not listed",
			setup: func(c *gin.Context) {
				c.Set(UserIDKey, caller)
				c.Set(OperatorAllowlistKey, map[uuid.UUID]struct{}{uuid.New(): {}})
			},
			wantAllow: false,
		},
		{
			name: "no authenticated caller",
			setup: func(c *gin.Context) {
				c.Set(OperatorAllowlistKey, map[uuid.UUID]struct{}{caller: {}})
			},
			wantAllow: false,
		},
		{
			name: "caller listed",
			setup: func(c *gin.Context) {
				c.Set(UserIDKey, caller)
				c.Set(OperatorAllowlistKey, map[uuid.UUID]struct{}{caller: {}})
			},
			wantAllow: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			engine := gin.New()
			var got bool
			engine.GET("/x", func(c *gin.Context) {
				tc.setup(c)
				got = IsOperator(c)
				c.Status(http.StatusOK)
			})
			engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x", nil))
			if got != tc.wantAllow {
				t.Fatalf("IsOperator=%v, want %v", got, tc.wantAllow)
			}
		})
	}
}
