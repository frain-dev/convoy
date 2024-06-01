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

func NewLimiter(keys []string, cfg config.Configuration, useMemory bool) (RateLimiter, error) {
	if useMemory {
		ml := mlimiter.NewMemoryRateLimiter(keys, int(cfg.PubSubIngestRate))
		return ml, nil
	}

	ra, err := rlimiter.NewRedisLimiter(cfg.Redis.BuildDsn())
	if err != nil {
		return nil, err
	}

	return ra, nil
}
