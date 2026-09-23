package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"go-starter-kit/infrastructure/config"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type HandlerDatabase struct {
	DB  *gorm.DB
	sql *sql.DB
}

func (h *HandlerDatabase) SQLDB() *sql.DB {
	if h == nil {
		return nil
	}
	return h.sql
}

// New creates a database connection based on the configured driver (postgres, mysql, sqlite).
func New(ctx context.Context, cfg config.DatabaseConfig) (*HandlerDatabase, error) {
	switch cfg.Driver {
	case "postgres":
		return newPostgres(ctx, cfg)
	case "mysql":
		return newMySQL(ctx, cfg)
	case "sqlite":
		return newSQLite(cfg)
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.Driver)
	}
}

func newPostgres(ctx context.Context, cfg config.DatabaseConfig) (*HandlerDatabase, error) {
	sqlDB, err := sql.Open("postgres", cfg.DSN())
	if err != nil {
		return nil, err
	}

	applyPoolConfig(sqlDB, cfg)

	if err := ping(ctx, sqlDB); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), gormConfig())
	if err != nil {
		_ = sqlDB.Close()
		return nil, err
	}

	return &HandlerDatabase{DB: db, sql: sqlDB}, nil
}

func newMySQL(ctx context.Context, cfg config.DatabaseConfig) (*HandlerDatabase, error) {
	sqlDB, err := sql.Open("mysql", cfg.DSN())
	if err != nil {
		return nil, err
	}

	applyPoolConfig(sqlDB, cfg)

	if err := ping(ctx, sqlDB); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}

	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB}), gormConfig())
	if err != nil {
		_ = sqlDB.Close()
		return nil, err
	}

	return &HandlerDatabase{DB: db, sql: sqlDB}, nil
}

func newSQLite(cfg config.DatabaseConfig) (*HandlerDatabase, error) {
	db, err := gorm.Open(sqlite.Open(cfg.DSN()), gormConfig())
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	// SQLite works best with limited connections
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(0)

	// Enable WAL mode for better concurrent read performance
	if err := db.Exec("PRAGMA journal_mode=WAL").Error; err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	if err := db.Exec("PRAGMA foreign_keys=ON").Error; err != nil {
		_ = sqlDB.Close()
		return nil, err
	}

	return &HandlerDatabase{DB: db, sql: sqlDB}, nil
}

func (h *HandlerDatabase) Close() error {
	if h == nil || h.sql == nil {
		return nil
	}
	return h.sql.Close()
}

func applyPoolConfig(sqlDB *sql.DB, cfg config.DatabaseConfig) {
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
}

func ping(ctx context.Context, sqlDB *sql.DB) error {
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return sqlDB.PingContext(pingCtx)
}

func gormConfig() *gorm.Config {
	return &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	}
}
