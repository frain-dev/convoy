package limiter

import (
	"context"
	"github.com/frain-dev/convoy/config"
	rlimiter "github.com/frain-dev/convoy/internal/pkg/limiter/redis"
)

type RateLimiter interface {
	// Allow rate limits outgoing events to endpoints based on a rate in a specified time duration by the endpoint id
	Allow(ctx context.Context, key string, rate int) error
	AllowWithDuration(ctx context.Context, key string, rate int, duration int) error
}

func NewLimiter(cfg config.Configuration) (RateLimiter, error) {
	r, err := rlimiter.NewRedisLimiter(cfg.Redis.BuildDsn())
	if err != nil {
		return nil, err
	}

	return r, nil
}
