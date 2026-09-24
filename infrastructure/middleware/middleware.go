package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"go-starter-kit/infrastructure/httplib"
	"go-starter-kit/infrastructure/jwt"
	"go-starter-kit/infrastructure/limiter"
	"go-starter-kit/modules/primitive"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type AuditRecorder interface {
	Record(ctx context.Context, log primitive.AuditLog) error
}

const UserIDKey = "user_id"

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.NewString()
		}
		c.Header("X-Request-ID", requestID)
		c.Set("request_id", requestID)
		c.Next()
	}
}

func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "no-referrer")
		c.Next()
	}
}

func RateLimiterMiddleware(rateLimiter limiter.RateLimiter) gin.HandlerFunc {
	return RateLimiterMiddlewareWithPrefix(rateLimiter, "global")
}

func RateLimiterMiddlewareWithPrefix(rateLimiter limiter.RateLimiter, prefix string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !rateLimiter.Allow(c.Request.Context(), prefix+":"+clientKey(c)) {
			httplib.SetErrorResponse(c, http.StatusTooManyRequests, "rate limit exceeded", nil)
			c.Abort()
			return
		}
		c.Next()
	}
}

func Recovery(log *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				requestID, _ := c.Get("request_id")
				log.WithFields(logrus.Fields{
					"request_id": requestID,
					"panic":      fmt.Sprintf("%v", recovered),
					"stack":      string(debug.Stack()),
				}).Error("panic recovered")
				httplib.SetErrorResponse(c, http.StatusInternalServerError, "internal server error", nil)
				c.Abort()
			}
		}()
		c.Next()
	}
}

func AccessLog(log *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		requestID, _ := c.Get("request_id")
		userID, _ := GetUserID(c)
		log.WithFields(logrus.Fields{
			"request_id": requestID,
			"latency_ms": time.Since(start).Milliseconds(),
			"method":     c.Request.Method,
			"route":      c.FullPath(),
			"path":       c.Request.URL.Path,
			"status":     c.Writer.Status(),
			"ip":         c.ClientIP(),
			"user_id":    userID.String(),
		}).Info("http request")
	}
}

func AuditLog(recorder AuditRecorder, log *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if recorder == nil || shouldSkipAudit(c) {
			return
		}

		requestID, _ := c.Get("request_id")
		var actorID *uuid.UUID
		if userID, ok := GetUserID(c); ok && userID != uuid.Nil {
			actorID = &userID
		}

		metadata, _ := json.Marshal(map[string]interface{}{
			"query": c.Request.URL.RawQuery,
		})

		entry := primitive.AuditLog{
			ActorID:      actorID,
			Action:       auditAction(c.Request.Method),
			ResourceType: auditResourceType(c.FullPath(), c.Request.URL.Path),
			ResourceID:   c.Param("id"),
			Method:       c.Request.Method,
			Path:         c.Request.URL.Path,
			Status:       c.Writer.Status(),
			IPAddress:    c.ClientIP(),
			UserAgent:    c.Request.UserAgent(),
			RequestID:    fmt.Sprintf("%v", requestID),
			Metadata:     string(metadata),
		}
		if err := recorder.Record(c.Request.Context(), entry); err != nil {
			log.WithError(err).Error("record audit log failed")
		}
	}
}

func BodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if maxBytes > 0 && c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}

func AuthMiddleware(tokens *jwt.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, err := tokens.ParseAccessToken(c.GetHeader("Authorization"))
		if err != nil {
			httplib.SetErrorResponse(c, http.StatusUnauthorized, "unauthorized", nil)
			c.Abort()
			return
		}
		c.Set(UserIDKey, claims.UserID)
		c.Next()
	}
}

func GetUserID(c *gin.Context) (uuid.UUID, bool) {
	value, exists := c.Get(UserIDKey)
	if !exists {
		return uuid.Nil, false
	}
	userID, ok := value.(uuid.UUID)
	return userID, ok
}

// OperatorAllowlistKey is the context key the operator gate reads. The value is
// a map[uuid.UUID]struct{} of accounts allowed to read operator-only surfaces
// (the user directory, the audit trail).
const OperatorAllowlistKey = "operator_allowlist"

// IsOperator reports whether the caller may read operator-only data.
//
// Fail-closed by construction: with no allowlist wired, nobody is an operator.
// That matters because this repository has no role column — Discord is the only
// place an "admin" is defined, and the HTTP surface has no other notion of one.
// An unauthenticated or unlisted caller therefore gets nothing rather than
// everything, which is the opposite of the previous behaviour.
func IsOperator(c *gin.Context) bool {
	value, exists := c.Get(OperatorAllowlistKey)
	if !exists {
		return false
	}
	allowlist, ok := value.(map[uuid.UUID]struct{})
	if !ok || len(allowlist) == 0 {
		return false
	}
	userID, ok := GetUserID(c)
	if !ok || userID == uuid.Nil {
		return false
	}
	_, allowed := allowlist[userID]
	return allowed
}

// OperatorMiddleware rejects any caller that is not in the operator allowlist.
// Use it to mount an entire route group that must never be reachable by an
// ordinary authenticated account. Like IsOperator it fails closed: with no
// allowlist wired, every request is refused.
func OperatorMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !IsOperator(c) {
			httplib.SetErrorResponse(c, http.StatusForbidden, primitive.MessageForbidden, nil)
			c.Abort()
			return
		}
		c.Next()
	}
}

func clientKey(c *gin.Context) string {
	if userID, ok := GetUserID(c); ok {
		return "user:" + userID.String()
	}
	// c.ClientIP() already resolves X-Forwarded-For, but only from the proxies
	// gin was told to trust (SetTrustedProxies). Reading the header directly here
	// would let any caller pick its own rate-limit bucket by sending a fresh
	// X-Forwarded-For on every request, which is a trivial bypass.
	return "ip:" + c.ClientIP()
}

func DrainAndClose(body io.ReadCloser) {
	if body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, body)
	_ = body.Close()
}

func shouldSkipAudit(c *gin.Context) bool {
	path := c.Request.URL.Path
	return strings.Contains(path, "/health") || strings.Contains(path, "/metrics")
}

func auditAction(method string) string {
	switch method {
	case http.MethodPost:
		return "create"
	case http.MethodPut, http.MethodPatch:
		return "update"
	case http.MethodDelete:
		return "delete"
	default:
		return "read"
	}
}

func auditResourceType(route, path string) string {
	value := route
	if value == "" {
		value = path
	}
	parts := strings.Split(strings.Trim(value, "/"), "/")
	if len(parts) >= 3 && parts[0] == "api" {
		return parts[2]
	}
	if len(parts) > 0 && parts[0] != "" {
		return parts[0]
	}
	return "unknown"
}
