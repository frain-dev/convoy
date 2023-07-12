package limiter

import (
	"context"
	"strings"

	"github.com/frain-dev/convoy/config"
	rlimiter "github.com/frain-dev/convoy/limiter/redis"
	"github.com/go-redis/redis_rate/v10"
)

type RateLimiter interface {
	Allow(ctx context.Context, key string, limit, duration int) (*redis_rate.Result, error)
	ShouldAllow(ctx context.Context, key string, limit, duration int) (*redis_rate.Result, error)
}

func NewLimiter(cfg config.RedisConfiguration) (RateLimiter, error) {
	addresses := strings.Split(cfg.Addresses, ",")
	ra, err := rlimiter.NewRedisLimiter(addresses)
	if err != nil {
		return nil, err
	}

	return ra, nil
}
