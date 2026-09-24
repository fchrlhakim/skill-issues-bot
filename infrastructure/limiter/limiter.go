package limiter

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
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
	// entries counts live buckets, since sync.Map has no Len.
	entries atomic.Int64
}

// maxLimiterEntries caps how many per-client buckets are held. Keys are derived
// from client IPs, so an attacker with many addresses could otherwise grow this
// map without limit. On overflow the whole map is dropped, which only resets
// in-flight counters; correctness of the limit itself is unaffected.
const maxLimiterEntries = 10_000

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
	if _, loaded := r.limiters.Load(key); !loaded && r.entries.Load() >= maxLimiterEntries {
		// Bounded memory beats precise counters for an unknown client set. A
		// dropped map only lets a burst through once, and the per-route limits
		// still apply.
		r.limiters.Range(func(k, _ any) bool {
			r.limiters.Delete(k)
			return true
		})
		r.entries.Store(0)
	}
	if _, loaded := r.limiters.LoadOrStore(key, rate.NewLimiter(rate.Limit(r.rps), r.burst)); !loaded {
		r.entries.Add(1)
	}
	value, ok := r.limiters.Load(key)
	if !ok {
		return false
	}
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
