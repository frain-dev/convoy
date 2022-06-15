package limiter

import (
	"context"

	"github.com/frain-dev/convoy/config"
	nlimiter "github.com/frain-dev/convoy/limiter/noop"
	rlimiter "github.com/frain-dev/convoy/limiter/redis"
	"github.com/go-redis/redis_rate/v9"
)

type RateLimiter interface {
	Allow(ctx context.Context, key string, limit, duration int) (*redis_rate.Result, error)
	ShouldAllow(ctx context.Context, key string, limit, duration int) (*redis_rate.Result, error)
}

func NewLimiter(cfg config.LimiterConfiguration) (RateLimiter, error) {
	if cfg.Type == config.RedisLimiterProvider {
		ra, err := rlimiter.NewRedisLimiter(cfg.Redis.Dsn)
		if err != nil {
			return nil, err
		}

		return ra, nil
	}
	return nlimiter.NewNoopLimiter(), nil
}
