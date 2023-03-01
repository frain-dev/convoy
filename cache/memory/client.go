package mcache

import (
	"context"
	"errors"
	"time"

	"github.com/go-redis/cache/v8"
)

type MemoryCache struct {
	cache *cache.Cache
}

const cacheSize = 128000

func NewMemoryCache() *MemoryCache {
	c := cache.New(&cache.Options{
		LocalCache: cache.NewTinyLFU(cacheSize, 0),
	})

	return &MemoryCache{cache: c}
}

func (m *MemoryCache) Set(ctx context.Context, key string, data interface{}, ttl time.Duration) error {
	return m.cache.Set(&cache.Item{
		Ctx:   ctx,
		Key:   key,
		Value: data,
		TTL:   ttl,
	})
}

func (m *MemoryCache) Get(ctx context.Context, key string, data interface{}) error {
	err := m.cache.Get(ctx, key, &data)

	if errors.Is(err, cache.ErrCacheMiss) {
		return nil
	}

	if err != nil {
		return err
	}

	return nil
}

func (m *MemoryCache) Delete(ctx context.Context, key string) error {
	return m.cache.Delete(ctx, key)
}
