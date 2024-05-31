package limiter

import (
	"context"
	"github.com/frain-dev/convoy/config"
	mlimiter "github.com/frain-dev/convoy/internal/pkg/limiter/memory"
	rlimiter "github.com/frain-dev/convoy/internal/pkg/limiter/redis"
)

type RateLimiter interface {
	// Allow rate limits outgoing events to endpoints based on a rate in a specified time duration by the endpoint id
	Allow(ctx context.Context, key string, rate int, duration int) error
}

func NewLimiter(cfg config.RedisConfiguration, useMemory bool) (RateLimiter, error) {
	ml := mlimiter.NewMemoryRateLimiter()

	if useMemory {
		ml = mlimiter.NewMemoryRateLimiter()
		return ml, nil
	}

	ra, err := rlimiter.NewRedisLimiter(cfg.BuildDsn())
	if err != nil {
		return nil, err
	}

	return ra, nil
}
