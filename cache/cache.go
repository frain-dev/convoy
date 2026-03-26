package cache

import (
	"context"
	"time"

	rcache "github.com/frain-dev/convoy/cache/redis"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
)

type Cache interface {
	Set(ctx context.Context, key string, data interface{}, expiration time.Duration) error
	Get(ctx context.Context, key string, data interface{}) error
	Delete(ctx context.Context, key string) error
}

func NewCache(cfg config.RedisConfiguration) (Cache, error) {
	client, err := rdb.NewClientFromRedisConfig(cfg)
	if err != nil {
		return nil, err
	}

	return rcache.NewRedisCacheFromClient(client.Client()), nil
}
