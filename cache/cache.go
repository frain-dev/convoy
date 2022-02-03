package cache

import (
	"context"
	"errors"
	"time"

	"github.com/frain-dev/convoy/config"
)

type Cache interface {
	Set(ctx context.Context, key string, data interface{}, expiration time.Duration) error
	Get(ctx context.Context, key string, data interface{}) (error, interface{})
	Delete(ctx context.Context, key string) error
}

func NewCache(cfg config.CacheConfiguration) (Cache, error) {
	if cfg.Type == "redis" {
		err, ca := NewRedisCache(cfg.Redis.DSN)
		if err != nil {
			return nil, err
		}

		return ca, nil
	}

	return nil, errors.New("Cache Type isn't supported")
}
