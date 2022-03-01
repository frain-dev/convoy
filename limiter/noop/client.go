package nooplimiter

import (
	"context"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/go-redis/redis_rate/v9"
)

type NoopLimiter struct {
}

func NewNoopLimiter(dsn string) (*NoopLimiter, error) {
	return &NoopLimiter{}, nil
}

func (n NoopLimiter) Allow(ctx context.Context, key string, limit int) (*redis_rate.Result, error) {
	return &redis_rate.Result{
			Limit:      redis_rate.PerMinute(convoy.RATE_LIMIT),
			Allowed:    convoy.RATE_LIMIT,
			Remaining:  convoy.RATE_LIMIT,
			RetryAfter: -1,
			ResetAfter: time.Minute,
		},
		nil
}
