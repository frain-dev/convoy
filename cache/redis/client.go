package rcache

import (
	"context"
	"errors"
	"time"

	"github.com/go-redis/cache/v9"
	"github.com/redis/go-redis/v9"

	"github.com/frain-dev/convoy/internal/pkg/rdb"
)

const cacheSize = 128000

type RedisCache struct {
	cache  *cache.Cache
	client redis.UniversalClient
}

func NewRedisCacheFromClient(rediser redis.UniversalClient) *RedisCache {
	c := cache.New(&cache.Options{Redis: rediser})
	return &RedisCache{cache: c, client: rediser}
}

func NewRedisCache(addresses []string) (*RedisCache, error) {
	client, err := rdb.NewClient(addresses)
	if err != nil {
		return nil, err
	}

	c := cache.New(&cache.Options{
		Redis:      client.Client(),
		LocalCache: cache.NewTinyLFU(cacheSize, 1*time.Minute),
	})

	r := &RedisCache{cache: c, client: client.Client()}

	return r, nil
}

func NewRedisCacheFromConfig(addresses []string, tlsSkipVerify bool, caCertFile, certFile, keyFile string) (*RedisCache, error) {
	client, err := rdb.NewClientFromConfig(addresses, tlsSkipVerify, caCertFile, certFile, keyFile)
	if err != nil {
		return nil, err
	}

	c := cache.New(&cache.Options{
		Redis:      client.Client(),
		LocalCache: cache.NewTinyLFU(cacheSize, 1*time.Minute),
	})

	r := &RedisCache{cache: c, client: client.Client()}

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

func (r *RedisCache) GetStrict(ctx context.Context, key string, data interface{}) error {
	err := r.cache.Get(ctx, key, data)
	if errors.Is(err, cache.ErrCacheMiss) {
		return nil
	}
	return err
}

func (r *RedisCache) GetOrCreateBytes(ctx context.Context, key string, value []byte) ([]byte, error) {
	created, err := r.client.SetNX(ctx, key, value, 0).Result()
	if err != nil {
		return nil, err
	}
	if created {
		return value, nil
	}
	return r.client.Get(ctx, key).Bytes()
}

func (r *RedisCache) Delete(ctx context.Context, key string) error {
	return r.cache.Delete(ctx, key)
}
