package router

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go-starter-kit/boot"
	"go-starter-kit/infrastructure/config"
	"go-starter-kit/infrastructure/jwt"
	"go-starter-kit/infrastructure/limiter"
	"go-starter-kit/modules/membership"
	"go-starter-kit/modules/primitive"
	"go-starter-kit/modules/ticket"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// noopHttp satisfies every module HttpInterface the router mounts except the
// ticket one. The test exercises route middleware, so the handlers themselves
// are irrelevant; each group registers one path so it is non-empty.
type noopHttp struct{}

func (noopHttp) GroupHealth(g *gin.RouterGroup)     { g.Any("/x", func(*gin.Context) {}) }
func (noopHttp) GroupAuth(g *gin.RouterGroup)       { g.Any("/x", func(*gin.Context) {}) }
func (noopHttp) GroupUser(g *gin.RouterGroup)       { g.Any("/x", func(*gin.Context) {}) }
func (noopHttp) GroupUpload(g *gin.RouterGroup)     { g.Any("/x", func(*gin.Context) {}) }
func (noopHttp) GroupAuditLog(g *gin.RouterGroup)   { g.Any("/x", func(*gin.Context) {}) }
func (noopHttp) GroupMembership(g *gin.RouterGroup) { g.Any("/x", func(*gin.Context) {}) }

// membershipStub records whether the request reached the group, which is how
// the operator gate is distinguished from "route missing".
type membershipStub struct{ reached bool }

func (m *membershipStub) GroupMembership(g *gin.RouterGroup) {
	g.POST("/verify", func(c *gin.Context) { m.reached = true })
}
func (m *membershipStub) Upsert(context.Context, string, string, string, int, bool) (primitive.GuildMember, error) {
	return primitive.GuildMember{}, nil
}
func (m *membershipStub) Find(context.Context, string) (primitive.GuildMember, error) {
	return primitive.GuildMember{}, nil
}
func (m *membershipStub) Verify(context.Context, string, []string, int) (membership.VerifyDecision, error) {
	return membership.VerifyDecision{}, nil
}
func (m *membershipStub) Snapshot(context.Context) (membership.ServerSnapshot, error) {
	return membership.ServerSnapshot{}, nil
}

// newTestRouter builds the real router with stubs for every surface the ticket
// gate does not depend on, plus a real ticket HTTP handler.
func newTestRouter(t *testing.T, allowlist []uuid.UUID) (*gin.Engine, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	tokens := jwt.NewService(strings.Repeat("k", 32), time.Minute, time.Hour)
	caller := uuid.New()
	pair, err := tokens.GeneratePair(caller)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	stub := noopHttp{}
	r := NewHandlerRouter(boot.HandlerSetup{
		Config: config.Config{
			Env:               "test",
			AllowedOrigins:    []string{"http://localhost:5173"},
			OperatorAllowlist: allowlist,
		},
		Logger:         logrus.New(),
		Limiter:        limiter.NewLocalRateLimiter(1000, 1000),
		Token:          tokens,
		HealthHttp:     stub,
		AuthHttp:       stub,
		UserHttp:       stub,
		UploadHttp:     stub,
		AuditHttp:      stub,
		MembershipHttp: stub,
		TicketHttp:     ticket.NewHttp(ticket.NewService(nil), nil),
	}).RouterWithMiddleware()

	return r, pair.AccessToken
}

// The ticket API takes its whole Actor - id, roles, is_admin, owner_id - from
// the request body. Before the operator gate was mounted on the group, any
// authenticated account could post is_admin:true and be treated as a Discord
// administrator, which reaches the withdrawal money path. The gate is the only
// thing between a plain JWT and that surface, so assert it at the route-table
// level: a test of the middleware alone would stay green if the group ever
// stopped mounting it.
func TestTicketRoutesRequireOperator(t *testing.T) {
	body := `{"actor":{"id":"attacker","is_admin":true,"owner_id":"attacker","available_admins":["attacker","other"]}}`
	cases := []struct {
		name, method, path string
	}{
		{"create", http.MethodPost, "/api/v1/tickets"},
		{"claim", http.MethodPost, "/api/v1/tickets/GEN-ID-x-00001/claim"},
		{"withdraw paid", http.MethodPost, "/api/v1/tickets/GEN-ID-x-00001/withdraw-paid"},
		{"mutasi", http.MethodGet, "/api/v1/tickets/mutasi?seller_id=attacker"},
		{"whoami", http.MethodGet, "/api/v1/tickets/whoami?is_admin=true"},
	}

	for _, tc := range cases {
		for _, allow := range []struct {
			name      string
			allowlist []uuid.UUID
		}{
			{"unlisted", []uuid.UUID{uuid.New()}},
			{"unset", nil},
		} {
			t.Run(tc.name+" "+allow.name, func(t *testing.T) {
				r, token := newTestRouter(t, allow.allowlist)
				req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer "+token)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
				if w.Code != http.StatusForbidden {
					t.Fatalf("status = %d, want 403; body=%s", w.Code, w.Body.String())
				}
			})
		}
	}
}

// Membership carries the same trust model: discord_id and roles come from the
// caller, so it is operator-only too.
func TestMembershipRoutesRequireOperator(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tokens := jwt.NewService(strings.Repeat("k", 32), time.Minute, time.Hour)
	caller := uuid.New()
	pair, err := tokens.GeneratePair(caller)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	stub := noopHttp{}
	guarded := &membershipStub{}
	r := NewHandlerRouter(boot.HandlerSetup{
		Config: config.Config{
			Env:            "test",
			AllowedOrigins: []string{"http://localhost:5173"},
		},
		Logger:         logrus.New(),
		Limiter:        limiter.NewLocalRateLimiter(1000, 1000),
		Token:          tokens,
		HealthHttp:     stub,
		AuthHttp:       stub,
		UserHttp:       stub,
		UploadHttp:     stub,
		AuditHttp:      stub,
		TicketHttp:     ticket.NewHttp(ticket.NewService(nil), nil),
		MembershipHttp: guarded,
	}).RouterWithMiddleware()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/membership/verify",
		strings.NewReader(`{"discord_id":"attacker","roles":["Verified"],"account_age_days":99}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body=%s", w.Code, w.Body.String())
	}
	if guarded.reached {
		t.Fatal("request reached the membership handler despite no operator allowlist")
	}
}

// The gate must not be a blanket denial: an allowlisted operator still reaches
// the ticket surface.
func TestTicketRoutesAllowListedOperator(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tokens := jwt.NewService(strings.Repeat("k", 32), time.Minute, time.Hour)
	caller := uuid.New()
	pair, err := tokens.GeneratePair(caller)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	stub := noopHttp{}
	r := NewHandlerRouter(boot.HandlerSetup{
		Config: config.Config{
			Env:               "test",
			AllowedOrigins:    []string{"http://localhost:5173"},
			OperatorAllowlist: []uuid.UUID{caller},
		},
		Logger:         logrus.New(),
		Limiter:        limiter.NewLocalRateLimiter(1000, 1000),
		Token:          tokens,
		HealthHttp:     stub,
		AuthHttp:       stub,
		UserHttp:       stub,
		UploadHttp:     stub,
		AuditHttp:      stub,
		MembershipHttp: stub,
		TicketHttp:     ticket.NewHttp(ticket.NewService(nil), nil),
	}).RouterWithMiddleware()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets/whoami", nil)
	req.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var envelope struct {
		Success bool `json:"success"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil || !envelope.Success {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}
