package rlimiter

import (
	"context"

	"github.com/go-redis/redis/v8"
	"github.com/go-redis/redis_rate/v9"
)

type RedisLimiter struct {
	limiter *redis_rate.Limiter
}

func NewRedisLimiter(dsn string) (*RedisLimiter, error) {
	opts, err := redis.ParseURL(dsn)

	if err != nil {
		return nil, err
	}

	client := redis.NewClient(opts)

	c := redis_rate.NewLimiter(client)

	r := &RedisLimiter{limiter: c}

	return r, nil
}

func (r RedisLimiter) Allow(ctx context.Context, key string, limit int) (*redis_rate.Result, error) {
	result, err := r.limiter.Allow(ctx, key, redis_rate.PerMinute(limit))
	if err != nil {
		return &redis_rate.Result{}, err
	}

	return result, nil
}
