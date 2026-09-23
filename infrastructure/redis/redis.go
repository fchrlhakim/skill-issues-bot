package redis

import (
	"context"
	"time"

	"go-starter-kit/infrastructure/config"

	goRedis "github.com/redis/go-redis/v9"
)

type Client struct {
	Redis *goRedis.Client
}

func NewRedisClient(ctx context.Context, cfg config.RedisConfig) (*Client, error) {
	client := goRedis.NewClient(&goRedis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}

	return &Client{Redis: client}, nil
}

func (c *Client) Close() error {
	if c == nil || c.Redis == nil {
		return nil
	}
	return c.Redis.Close()
}
