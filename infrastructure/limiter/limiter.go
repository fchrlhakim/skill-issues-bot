package limiter

import (
	"context"
	"fmt"
	"sync"
	"time"

	goRedis "github.com/redis/go-redis/v9"
	"golang.org/x/time/rate"
)

type RateLimiter interface {
	Allow(ctx context.Context, key string) bool
}

type LocalRateLimiter struct {
	limiters sync.Map
	rps      float64
	burst    int
	timeout  time.Duration
}

func NewLocalRateLimiter(rps float64, burst int) RateLimiter {
	if rps <= 0 {
		rps = 1
	}
	if burst <= 0 {
		burst = 1
	}
	return &LocalRateLimiter{
		rps:     rps,
		burst:   burst,
		timeout: 100 * time.Millisecond,
	}
}

func (r *LocalRateLimiter) Allow(ctx context.Context, key string) bool {
	if r == nil {
		return true
	}
	if key == "" {
		key = "global"
	}
	value, _ := r.limiters.LoadOrStore(key, rate.NewLimiter(rate.Limit(r.rps), r.burst))
	limiter, ok := value.(*rate.Limiter)
	if !ok {
		return false
	}
	waitCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	return limiter.Wait(waitCtx) == nil
}

type RedisRateLimiter struct {
	redis  *goRedis.Client
	limit  int
	window time.Duration
}

func NewRedisRateLimiter(redis *goRedis.Client, limit int, window time.Duration) RateLimiter {
	if limit <= 0 {
		limit = 1
	}
	if window <= 0 {
		window = time.Second
	}
	return &RedisRateLimiter{redis: redis, limit: limit, window: window}
}

func (r *RedisRateLimiter) Allow(ctx context.Context, key string) bool {
	if r == nil || r.redis == nil {
		return true
	}
	redisKey := fmt.Sprintf("rate_limit:%s", key)
	count, err := r.redis.Incr(ctx, redisKey).Result()
	if err != nil {
		return false
	}
	if count == 1 {
		_ = r.redis.Expire(ctx, redisKey, r.window).Err()
	}
	return count <= int64(r.limit)
}
