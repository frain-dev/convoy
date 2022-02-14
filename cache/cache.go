package cache

import (
	"context"
	"time"

	"github.com/frain-dev/convoy/cache/memory"
	"github.com/frain-dev/convoy/cache/redis"
	"github.com/frain-dev/convoy/config"
)

type Cache interface {
	Set(ctx context.Context, key string, data interface{}, expiration time.Duration) error
	Get(ctx context.Context, key string, data interface{}) error
	Delete(ctx context.Context, key string) error
}

func NewCache(cfg config.CacheConfiguration) (Cache, error) {
	if cfg.Type == config.RedisCacheProvider {
		ca, err := rcache.NewRedisCache(cfg.Redis.Dsn)
		if err != nil {
			return nil, err
		}

		return ca, nil
	}

	return mcache.NewMemoryCache(), nil

}
