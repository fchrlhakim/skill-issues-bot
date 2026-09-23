package boot

import (
	"context"
	"time"

	"go-starter-kit/infrastructure/config"
	"go-starter-kit/infrastructure/database"
	"go-starter-kit/infrastructure/jwt"
	"go-starter-kit/infrastructure/limiter"
	logger "go-starter-kit/infrastructure/log"
	"go-starter-kit/infrastructure/observability"
	"go-starter-kit/infrastructure/redis"
	"go-starter-kit/infrastructure/storage"
	"go-starter-kit/infrastructure/validator"
	auditLog "go-starter-kit/modules/audit-log"
	"go-starter-kit/modules/auth"
	"go-starter-kit/modules/health"
	"go-starter-kit/modules/membership"
	"go-starter-kit/modules/ticket"
	"go-starter-kit/modules/upload"
	"go-starter-kit/modules/user"

	"github.com/sirupsen/logrus"
)

type HandlerSetup struct {
	Config         config.Config
	Logger         *logrus.Logger
	Limiter        limiter.RateLimiter
	Token          *jwt.Service
	AuthHttp       auth.HttpInterface
	AuditLog       auditLog.ServiceInterface
	AuditHttp      auditLog.HttpInterface
	HealthHttp     health.HttpInterface
	UploadHttp     upload.HttpInterface
	UserHttp       user.HttpInterface
	TicketHttp     ticket.HttpInterface
	MembershipHttp membership.HttpInterface
	TicketService  ticket.ServiceInterface
}

func MakeHandler(cfg config.Config) (HandlerSetup, func(), error) {
	log := logger.New(cfg.LogLevel, cfg.LogFormat)
	db, err := database.New(context.Background(), cfg.Database)
	if err != nil {
		return HandlerSetup{}, func() {}, err
	}
	if cfg.Metrics.Enable {
		observability.RegisterDBStats(db.SQLDB())
	}
	var redisClient *redis.Client
	if cfg.Redis.Enable {
		redisClient, err = redis.NewRedisClient(context.Background(), cfg.Redis)
		if err != nil {
			_ = db.Close()
			return HandlerSetup{}, func() {}, err
		}
	}

	tokenService := jwt.NewService(cfg.JWT.Secret, cfg.JWT.AccessTTL, cfg.JWT.RefreshTTL)
	rateLimiter := limiter.NewLocalRateLimiter(cfg.RateLimit.RPS, cfg.RateLimit.Burst)
	if cfg.RateLimit.Backend == "redis" && redisClient != nil {
		rateLimiter = limiter.NewRedisRateLimiter(redisClient.Redis, int(cfg.RateLimit.RPS), time.Second)
	}
	validate := validator.New()

	userRepository := user.NewRepository(db.DB)
	userService := user.NewService(userRepository)
	userHttp := user.NewHttp(userService)

	authRepository := auth.NewRepository(db.DB)
	authService := auth.NewService(authRepository, userService, tokenService)
	authHttp := auth.NewHttp(authService, validate)
	healthRepository := health.NewRepository(db.DB)
	healthService := health.NewService(healthRepository)
	healthHttp := health.NewHttp(healthService)
	auditRepository := auditLog.NewRepository(db.DB)
	auditService := auditLog.NewService(auditRepository)
	auditHttp := auditLog.NewHttp(auditService)
	uploadRepository := upload.NewRepository(db.DB)
	uploadStorage := storage.NewLocalStorage(cfg.Storage.LocalDir)
	uploadService := upload.NewService(uploadRepository, uploadStorage, cfg.MaxBodyBytes)
	uploadHttp := upload.NewHttp(uploadService)

	ticketRepository := ticket.NewRepository(db.DB)
	ticketService := ticket.NewService(ticketRepository)
	ticketHttp := ticket.NewHttp(ticketService, validate)
	membershipRepository := membership.NewRepository(db.DB)
	membershipService := membership.NewService(membershipRepository)
	membershipHttp := membership.NewHttp(membershipService)

	cleanup := func() {
		if redisClient != nil {
			if err := redisClient.Close(); err != nil {
				log.Errorf("close redis: %v", err)
			}
		}
		if err := db.Close(); err != nil {
			log.Errorf("close database: %v", err)
		}
	}

	return HandlerSetup{
		Config:         cfg,
		Logger:         log,
		Limiter:        rateLimiter,
		Token:          tokenService,
		AuthHttp:       authHttp,
		AuditLog:       auditService,
		AuditHttp:      auditHttp,
		HealthHttp:     healthHttp,
		UploadHttp:     uploadHttp,
		UserHttp:       userHttp,
		TicketHttp:     ticketHttp,
		MembershipHttp: membershipHttp,
		TicketService:  ticketService,
	}, cleanup, nil
}
