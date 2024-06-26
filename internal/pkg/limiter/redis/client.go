package rlimiter

import (
	"context"
	"errors"
	"time"

	"github.com/frain-dev/convoy/internal/pkg/rdb"
	"github.com/go-redis/redis_rate/v10"
)

var ErrRateLimitExceeded = errors.New("rate limit exceeded")

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

func (r *RedisLimiter) Allow(ctx context.Context, key string, limit int, duration int) error {
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
		return &RedisLimiterError{
			delay: result.RetryAfter + result.ResetAfter,
			err:   ErrRateLimitExceeded,
		}
	}

	return nil
}

type RedisLimiterError struct {
	delay time.Duration
	err   error
}

func (e *RedisLimiterError) Error() string {
	return e.err.Error()
}

func GetRetryAfter(err error) time.Duration {
	if rateLimitError, ok := err.(*RedisLimiterError); ok {
		return rateLimitError.delay
	}
	return time.Duration(0)
}

func GetRawError(err error) error {
	if rateLimitError, ok := err.(*RedisLimiterError); ok {
		return rateLimitError.err
	}
	return nil
}
