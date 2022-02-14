package rcache

import (
	"context"
	"errors"
	"time"

	"github.com/go-redis/cache/v8"
	"github.com/go-redis/redis/v8"
)

type RedisCache struct {
	cache *cache.Cache
}

func NewRedisCache(dsn string) (*RedisCache, error) {
	opts, err := redis.ParseURL(dsn)

	if err != nil {
		return nil, err
	}

	client := redis.NewClient(opts)

	c := cache.New(&cache.Options{
		Redis: client,
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
