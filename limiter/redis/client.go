package rlimiter

import (
	"context"
	"time"

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

func (r *RedisLimiter) Allow(ctx context.Context, key string, limit, duration int) (*redis_rate.Result, error) {
	var d time.Duration

	if duration == int(time.Hour) {
		d = time.Hour
	} else if duration == int(time.Minute) {
		d = time.Minute
	} else {
		d = time.Second
	}

	l := redis_rate.Limit{
		Period: d,
		Rate:   limit,
		Burst:  limit,
	}

	result, err := r.limiter.Allow(ctx, key, l)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (r *RedisLimiter) ShouldAllow(ctx context.Context, key string, limit, duration int) (*redis_rate.Result, error) {
	var d time.Duration

	if duration == int(time.Hour) {
		d = time.Hour
	} else if duration == int(time.Minute) {
		d = time.Minute
	} else {
		d = time.Second
	}

	l := redis_rate.Limit{
		Period: d,
		Rate:   limit,
		Burst:  limit,
	}

	result, err := r.limiter.AllowN(ctx, key, l, 0)
	if err != nil {
		return nil, err
	}

	return result, nil
}
