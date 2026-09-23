package cache

import (
	"context"
	"errors"
	"time"

	goRedis "github.com/redis/go-redis/v9"
)

var ErrCacheMiss = errors.New("cache miss")

type Cache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, keys ...string) error
}

type NoopCache struct{}

func NewNoopCache() Cache { return NoopCache{} }

func (NoopCache) Get(ctx context.Context, key string) ([]byte, error) { return nil, ErrCacheMiss }
func (NoopCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return nil
}
func (NoopCache) Delete(ctx context.Context, keys ...string) error { return nil }

type RedisCache struct {
	redis *goRedis.Client
}

func NewRedisCache(redis *goRedis.Client) Cache {
	return &RedisCache{redis: redis}
}

func (c *RedisCache) Get(ctx context.Context, key string) ([]byte, error) {
	value, err := c.redis.Get(ctx, key).Bytes()
	if errors.Is(err, goRedis.Nil) {
		return nil, ErrCacheMiss
	}
	return value, err
}

func (c *RedisCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return c.redis.Set(ctx, key, value, ttl).Err()
}

func (c *RedisCache) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return c.redis.Del(ctx, keys...).Err()
}
