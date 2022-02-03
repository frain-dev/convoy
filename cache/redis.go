package cache

import (
	"context"
	"time"

	"github.com/go-redis/cache/v8"
	"github.com/go-redis/redis/v8"
)

type RedisCache struct {
	cache *cache.Cache
}

func NewRedisCache(dsn string) (error, Cache) {
	opts, err := redis.ParseURL(dsn)

	if err != nil {
		return err, nil
	}

	client := redis.NewClient(opts)

	c := cache.New(&cache.Options{
		Redis:      client,
		LocalCache: cache.NewTinyLFU(1000, time.Minute),
	})

	r := &RedisCache{cache: c}

	return nil, r
}

func (r *RedisCache) Set(ctx context.Context, key string, data interface{}, ttl time.Duration) error {
	return r.cache.Set(&cache.Item{
		Ctx:   ctx,
		Key:   key,
		Value: data,
		TTL:   ttl,
	})
}

func (r *RedisCache) Get(ctx context.Context, key string, data interface{}) (error, interface{}) {
	err := r.cache.Get(ctx, key, &data)

	if err != nil {
		return err, nil
	}

	return nil, data
}

func (r *RedisCache) Delete(ctx context.Context, key string) error {
	return nil
}
