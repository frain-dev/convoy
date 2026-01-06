package cache

import (
	"context"
	"time"

	rcache "github.com/frain-dev/convoy/cache/redis"
	"github.com/frain-dev/convoy/config"
)

type Cache interface {
	Set(ctx context.Context, key string, data interface{}, expiration time.Duration) error
	Get(ctx context.Context, key string, data interface{}) error
	Delete(ctx context.Context, key string) error
}

func NewCache(cfg config.RedisConfiguration) (Cache, error) {
	ca, err := rcache.NewRedisCacheFromConfig(
		cfg.BuildDsn(),
		cfg.TLSSkipVerify,
		cfg.TLSCACertFile,
		cfg.TLSCertFile,
		cfg.TLSKeyFile,
	)
	if err != nil {
		return nil, err
	}

	return ca, nil
}
