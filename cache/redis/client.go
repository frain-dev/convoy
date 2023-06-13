package rcache

import (
	"context"
	"errors"
	"time"

	"github.com/frain-dev/convoy/internal/pkg/rdb"
	"github.com/go-redis/cache/v9"
)

type RedisCache struct {
	cache *cache.Cache
}

func NewRedisCache(dsn string) (*RedisCache, error) {
	rdb, err := rdb.NewClient(dsn)
	if err != nil {
		return nil, err
	}

	c := cache.New(&cache.Options{
		Redis: rdb.Client(),
	})

	r := &RedisCache{cache: c}

	return r, nil
}

func (r *RedisCache) Set(ctx context.Context, key string, data interface{}, ttl time.Duration) error {
	return r.cache.Set(&cache.Item{
		Ctx:   ctx,
		Key:   key,
		Value: data,
		TTL:   ttl,
	})
}

func (r *RedisCache) Get(ctx context.Context, key string, data interface{}) error {
	err := r.cache.Get(ctx, key, &data)

	if errors.Is(err, cache.ErrCacheMiss) {
		return nil
	}

	if err != nil {
		return err
	}

	return nil
}

func (r *RedisCache) Delete(ctx context.Context, key string) error {
	return r.cache.Delete(ctx, key)
}
