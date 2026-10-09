package rlimiter

import (
	"context"
	"time"

	"github.com/go-redis/redis_rate/v10"
	"github.com/redis/go-redis/v9"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/internal/pkg/limiter"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
)

var _ limiter.RateLimiter = (*RedisLimiter)(nil)

type RedisLimiter struct {
	limiter *redis_rate.Limiter
}

func NewLimiterFromRedisClient(rediser redis.UniversalClient) *RedisLimiter {
	return &RedisLimiter{limiter: redis_rate.NewLimiter(rediser)}
}

func NewRedisLimiter(addresses []string) (*RedisLimiter, error) {
	client, err := rdb.NewClient(addresses)
	if err != nil {
		return nil, err
	}

	c := redis_rate.NewLimiter(client.Client())
	r := &RedisLimiter{limiter: c}

	return r, nil
}

func NewRedisLimiterFromConfig(addresses []string, tlsSkipVerify bool, caCertFile, certFile, keyFile string) (*RedisLimiter, error) {
	client, err := rdb.NewClientFromConfig(addresses, tlsSkipVerify, caCertFile, certFile, keyFile)
	if err != nil {
		return nil, err
	}

	c := redis_rate.NewLimiter(client.Client())
	r := &RedisLimiter{limiter: c}

	return r, nil
}

func NewRedisLimiterFromRedisConfig(cfg config.RedisConfiguration) (*RedisLimiter, error) {
	client, err := rdb.NewClientFromRedisConfig(cfg)
	if err != nil {
		return nil, err
	}

	c := redis_rate.NewLimiter(client.Client())
	r := &RedisLimiter{limiter: c}

	return r, nil
}

func (r *RedisLimiter) Allow(ctx context.Context, key string, limit int) error {
	l := redis_rate.Limit{
		Period: time.Second,
		Rate:   limit,
		Burst:  limit,
	}

	result, err := r.limiter.Allow(ctx, key, l)
	if err != nil {
		return err
	}

	if result.Remaining == 0 && result.RetryAfter > 0 {
		return limiter.NewRateLimitExceeded(result.RetryAfter)
	}

	return nil
}

func (r *RedisLimiter) AllowWithDuration(ctx context.Context, key string, limit, duration int) error {
	if limit == 0 || duration == 0 { // this should never happen
		return nil
	}

	l := redis_rate.Limit{
		Period: time.Second * time.Duration(duration),
		Rate:   limit,
		Burst:  limit,
	}

	result, err := r.limiter.Allow(ctx, key, l)
	if err != nil {
		return err
	}

	if result.Remaining == 0 && result.RetryAfter > 0 {
		return limiter.NewRateLimitExceeded(result.RetryAfter)
	}

	return nil
}
