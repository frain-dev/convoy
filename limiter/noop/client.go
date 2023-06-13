package nooplimiter

import (
	"context"
	"time"

	"github.com/go-redis/redis_rate/v10"
)

type NoopLimiter struct{}

func NewNoopLimiter() *NoopLimiter {
	return &NoopLimiter{}
}

func (n NoopLimiter) Allow(ctx context.Context, key string, limit, duration int) (*redis_rate.Result, error) {
	return &redis_rate.Result{
			Limit:      redis_rate.PerMinute(5000),
			Allowed:    5000,
			Remaining:  5000,
			RetryAfter: -1,
			ResetAfter: time.Minute,
		},
		nil
}

func (n NoopLimiter) ShouldAllow(ctx context.Context, key string, limit, duration int) (*redis_rate.Result, error) {
	return &redis_rate.Result{
			Limit:      redis_rate.PerMinute(5000),
			Allowed:    5000,
			Remaining:  5000,
			RetryAfter: -1,
			ResetAfter: time.Minute,
		},
		nil
}
