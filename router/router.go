package router

import (
	"net/http"
	"time"

	"go-starter-kit/boot"
	"go-starter-kit/infrastructure/httplib"
	"go-starter-kit/infrastructure/middleware"
	"go-starter-kit/infrastructure/observability"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

type HandlerRouter struct {
	Setup boot.HandlerSetup
}

type InterfaceRouter interface {
	RouterWithMiddleware() *gin.Engine
}

func NewHandlerRouter(setup boot.HandlerSetup) InterfaceRouter {
	return &HandlerRouter{Setup: setup}
}

func (hr *HandlerRouter) RouterWithMiddleware() *gin.Engine {
	if hr.Setup.Config.Env == "prod" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(middleware.Recovery(hr.Setup.Logger))
	r.Use(gzip.Gzip(gzip.DefaultCompression))
	r.Use(middleware.AccessLog(hr.Setup.Logger))
	r.Use(middleware.RequestID())
	r.Use(middleware.SecurityHeaders())
	r.Use(middleware.BodyLimit(hr.Setup.Config.MaxBodyBytes))
	r.Use(middleware.RateLimiterMiddlewareWithPrefix(hr.Setup.Limiter, "global"))
	if hr.Setup.Config.Metrics.Enable {
		r.Use(observability.Middleware())
	}
	r.Use(middleware.AuditLog(hr.Setup.AuditLog, hr.Setup.Logger))
	r.Use(cors.New(cors.Config{
		AllowOrigins:     hr.Setup.Config.AllowedOrigins,
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowHeaders:     []string{"Authorization", "Content-Type", "X-Request-ID"},
		ExposeHeaders:    []string{"X-Request-ID"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))

	r.NoRoute(func(c *gin.Context) {
		httplib.SetErrorResponse(c, http.StatusNotFound, "route not found", nil)
	})
	r.NoMethod(func(c *gin.Context) {
		httplib.SetErrorResponse(c, http.StatusMethodNotAllowed, "method not allowed", nil)
	})

	api := r.Group("/api")
	v1 := api.Group("/v1")
	if hr.Setup.Config.Metrics.Enable {
		v1.GET("/metrics", observability.Handler())
	}

	hr.Setup.HealthHttp.GroupHealth(v1.Group("/health"))
	authGroup := v1.Group("/auth")
	authGroup.Use(middleware.RateLimiterMiddlewareWithPrefix(hr.Setup.Limiter, "auth"))
	hr.Setup.AuthHttp.GroupAuth(authGroup)

	userGroup := v1.Group("/user")
	userGroup.Use(middleware.AuthMiddleware(hr.Setup.Token))
	userGroup.Use(middleware.RateLimiterMiddlewareWithPrefix(hr.Setup.Limiter, "user"))
	hr.Setup.UserHttp.GroupUser(userGroup)
	uploadGroup := v1.Group("/uploads")
	uploadGroup.Use(middleware.AuthMiddleware(hr.Setup.Token))
	uploadGroup.Use(middleware.RateLimiterMiddlewareWithPrefix(hr.Setup.Limiter, "upload"))
	hr.Setup.UploadHttp.GroupUpload(uploadGroup)
	auditGroup := v1.Group("/audit-logs")
	auditGroup.Use(middleware.AuthMiddleware(hr.Setup.Token))
	auditGroup.Use(middleware.RateLimiterMiddlewareWithPrefix(hr.Setup.Limiter, "audit"))
	hr.Setup.AuditHttp.GroupAuditLog(auditGroup)

	ticketGroup := v1.Group("/tickets")
	ticketGroup.Use(middleware.AuthMiddleware(hr.Setup.Token))
	ticketGroup.Use(middleware.RateLimiterMiddlewareWithPrefix(hr.Setup.Limiter, "ticket"))
	hr.Setup.TicketHttp.GroupTicket(ticketGroup)

	memberGroup := v1.Group("/membership")
	memberGroup.Use(middleware.AuthMiddleware(hr.Setup.Token))
	memberGroup.Use(middleware.RateLimiterMiddlewareWithPrefix(hr.Setup.Limiter, "membership"))
	hr.Setup.MembershipHttp.GroupMembership(memberGroup)

	return r
}
