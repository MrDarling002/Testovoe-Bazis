// Package ratelimit implements a token-bucket rate limiter backed by an
// atomic Redis Lua script.
package ratelimit

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type Limiter interface {
	// Allow reports whether the request identified by key is within the
	// limit. On infrastructure errors it returns (false, err); the caller
	// decides whether to fail open or closed.
	Allow(ctx context.Context, key string) (bool, error)
}

type RedisLimiter struct {
	rdb    *redis.Client
	script *redis.Script
	limit  int
}

func NewRedisLimiter(rdb *redis.Client, requestsPerMinute int) *RedisLimiter {
	return &RedisLimiter{
		rdb:   rdb,
		limit: requestsPerMinute,
		script: redis.NewScript(`
			local key = KEYS[1]
			local capacity = tonumber(ARGV[1])
			local refill_per_second = tonumber(ARGV[2])
			local requested = tonumber(ARGV[3])

			local time = redis.call('TIME')
			local now = tonumber(time[1]) + tonumber(time[2]) / 1000000

			local data = redis.call('HMGET', key, 'tokens', 'ts')
			local tokens = tonumber(data[1])
			local ts = tonumber(data[2])

			if tokens == nil then
				tokens = capacity
				ts = now
			end

			local elapsed = math.max(0, now - ts)
			tokens = math.min(capacity, tokens + elapsed * refill_per_second)

			local allowed = 0

			if tokens >= requested then
				allowed = 1
				tokens = tokens - requested
			end

			redis.call('HSET', key, 'tokens', tokens, 'ts', now)
			redis.call('EXPIRE', key, math.ceil(capacity / refill_per_second) + 10)

			return allowed
		`),
	}
}

func (l *RedisLimiter) Allow(ctx context.Context, key string) (bool, error) {
	refillPerSecond := float64(l.limit) / 60.0

	res, err := l.script.Run(ctx, l.rdb, []string{key}, l.limit, refillPerSecond, 1).Result()
	if err != nil {
		return false, fmt.Errorf("run rate limit script: %w", err)
	}

	allowed, ok := res.(int64)
	if !ok {
		return false, fmt.Errorf("unexpected rate limit script result type %T", res)
	}

	return allowed == 1, nil
}
