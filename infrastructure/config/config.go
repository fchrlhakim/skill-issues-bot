package config

import (
	"errors"
	"fmt"
	"github.com/google/uuid"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Env            string
	Name           string
	Port           string
	LogLevel       string
	LogFormat      string
	AllowedOrigins []string
	// TrustedProxies lists the CIDRs whose X-Forwarded-For may be believed.
	// Empty means "trust none", which is correct for a service published only on
	// loopback: the peer address is then the real client.
	TrustedProxies []string
	// OperatorAllowlist holds the user ids allowed to read operator-only data
	// (the user directory and the audit trail). Empty means nobody, which is the
	// safe default: this repo has no role column, so there is no other way to
	// identify an operator.
	OperatorAllowlist []uuid.UUID
	HTTP              HTTPConfig
	JWT               JWTConfig
	Database          DatabaseConfig
	Redis             RedisConfig
	HTTPClient        HTTPClientConfig
	RateLimit         RateLimitConfig
	Storage           StorageConfig
	Metrics           MetricsConfig
	Discord           DiscordConfig
	MaxBodyBytes      int64
	MaxMultipart      int64
}

type HTTPConfig struct {
	ReadTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
}

type JWTConfig struct {
	Secret     string
	AccessTTL  time.Duration
	RefreshTTL time.Duration
}

type DatabaseConfig struct {
	Driver          string // postgres, mysql, sqlite
	Host            string
	Port            string
	DBName          string
	User            string
	Password        string
	SSLMode         string // postgres only
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

type RateLimitConfig struct {
	RPS     float64
	Burst   int
	Backend string
}

type RedisConfig struct {
	Enable   bool
	Addr     string
	Password string
	DB       int
}

type HTTPClientConfig struct {
	Timeout time.Duration
}

type StorageConfig struct {
	Driver   string
	LocalDir string
}

type MetricsConfig struct {
	Enable bool
}

type DiscordConfig struct {
	BotToken string
	GuildID  string
}

func Load() (Config, error) {
	_ = godotenv.Load()

	cfg := Config{
		Env:               getEnv("APP_ENV", "local"),
		Name:              getEnv("APP_NAME", "go-starter-kit"),
		Port:              getEnv("APP_PORT", "8080"),
		LogLevel:          getEnv("APP_LOG_LEVEL", "info"),
		LogFormat:         getEnv("APP_LOG_FORMAT", "json"),
		AllowedOrigins:    splitCSV(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")),
		TrustedProxies:    splitCSV(getEnv("TRUSTED_PROXIES", "")),
		OperatorAllowlist: parseUUIDList(getEnv("OPERATOR_USER_IDS", "")),
		HTTP: HTTPConfig{
			ReadTimeout:       getDuration("HTTP_READ_TIMEOUT", 10*time.Second),
			ReadHeaderTimeout: getDuration("HTTP_READ_HEADER_TIMEOUT", 5*time.Second),
			WriteTimeout:      getDuration("HTTP_WRITE_TIMEOUT", 10*time.Second),
			IdleTimeout:       getDuration("HTTP_IDLE_TIMEOUT", 60*time.Second),
			ShutdownTimeout:   getDuration("HTTP_SHUTDOWN_TIMEOUT", 10*time.Second),
		},
		JWT: JWTConfig{
			Secret:     getEnv("JWT_SECRET", ""),
			AccessTTL:  getDuration("JWT_ACCESS_TTL", 15*time.Minute),
			RefreshTTL: getDuration("JWT_REFRESH_TTL", 30*24*time.Hour),
		},
		Database: DatabaseConfig{
			Driver:          getEnv("DB_DRIVER", "postgres"),
			Host:            getEnv("DB_HOST", "localhost"),
			Port:            getEnv("DB_PORT", "5432"),
			DBName:          getEnv("DB_NAME", "starter"),
			User:            getEnv("DB_USER", "starter"),
			Password:        getEnv("DB_PASSWORD", "starter"),
			SSLMode:         getEnv("DB_SSL_MODE", "disable"),
			MaxOpenConns:    getInt("DB_MAX_OPEN_CONNS", 10),
			MaxIdleConns:    getInt("DB_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: getDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute),
		},
		Redis: RedisConfig{
			Enable:   getBool("REDIS_ENABLE", false),
			Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getInt("REDIS_DB", 0),
		},
		HTTPClient: HTTPClientConfig{
			Timeout: getDuration("HTTP_CLIENT_TIMEOUT", 10*time.Second),
		},
		RateLimit: RateLimitConfig{
			RPS:     getFloat("RATE_LIMIT_RPS", 10),
			Burst:   getInt("RATE_LIMIT_BURST", 20),
			Backend: getEnv("RATE_LIMIT_BACKEND", "local"),
		},
		Storage: StorageConfig{
			Driver:   getEnv("STORAGE_DRIVER", "local"),
			LocalDir: getEnv("STORAGE_LOCAL_DIR", "storage"),
		},
		Metrics: MetricsConfig{
			Enable: getBool("METRICS_ENABLE", true),
		},
		Discord: DiscordConfig{
			BotToken: getEnv("DISCORD_BOT_TOKEN", ""),
			GuildID:  getEnv("DISCORD_GUILD_ID", ""),
		},
		MaxBodyBytes: getInt64("MAX_BODY_BYTES", 1<<20),
		MaxMultipart: getInt64("MAX_MULTIPART_MEMORY", 8<<20),
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if c.JWT.Secret == "" {
		return errors.New("JWT_SECRET is required")
	}
	if len(c.JWT.Secret) < 32 {
		return errors.New("JWT_SECRET must be at least 32 characters")
	}
	validDrivers := map[string]bool{"postgres": true, "mysql": true, "sqlite": true}
	if !validDrivers[c.Database.Driver] {
		return fmt.Errorf("DB_DRIVER must be one of: postgres, mysql, sqlite (got %q)", c.Database.Driver)
	}
	if c.Database.Driver != "sqlite" && c.Database.Password == "" {
		return errors.New("DB_PASSWORD is required for non-sqlite drivers")
	}
	if c.Port == "" {
		return errors.New("APP_PORT is required")
	}
	return nil
}

func (d DatabaseConfig) DSN() string {
	switch d.Driver {
	case "mysql":
		return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Asia%%2FJakarta",
			d.User, d.Password, d.Host, d.Port, d.DBName)
	case "sqlite":
		if d.DBName == "" {
			return "starter.db"
		}
		return d.DBName
	default: // postgres
		return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=Asia/Jakarta",
			d.Host, d.Port, d.User, d.Password, d.DBName, d.SSLMode)
	}
}

func getEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getInt64(key string, fallback int64) int64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func getBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getFloat(key string, fallback float64) float64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

// parseUUIDList reads a comma-separated allowlist, skipping anything that is not
// a UUID. A malformed entry is dropped rather than widening access.
func parseUUIDList(value string) []uuid.UUID {
	var ids []uuid.UUID
	for _, part := range splitCSV(value) {
		if parsed, err := uuid.Parse(part); err == nil && parsed != uuid.Nil {
			ids = append(ids, parsed)
		}
	}
	return ids
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
