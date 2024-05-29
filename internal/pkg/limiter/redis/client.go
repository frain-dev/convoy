package rlimiter

import (
	"context"
	"time"

	"github.com/frain-dev/convoy/internal/pkg/rdb"
	"github.com/go-redis/redis_rate/v10"
)

type RedisLimiter struct {
	limiter *redis_rate.Limiter
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

func (r *RedisLimiter) Allow(ctx context.Context, key string, limit int, duration time.Duration) (*redis_rate.Result, error) {
	l := redis_rate.Limit{
		Period: duration,
		Rate:   limit,
		Burst:  limit,
	}

	result, err := r.limiter.Allow(ctx, key, l)
	if err != nil {
		return nil, err
	}

	return result, nil
}
