package mcache

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/go-redis/cache/v9"
)

type MemoryCache struct {
	cache *cache.Cache
	mu    sync.Mutex
}

const cacheSize = 128000

func NewMemoryCache() *MemoryCache {
	c := cache.New(&cache.Options{
		LocalCache: cache.NewTinyLFU(cacheSize, time.Second),
	})

	return &MemoryCache{cache: c}
}

func (m *MemoryCache) Set(ctx context.Context, key string, data interface{}, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cache.Set(&cache.Item{
		Ctx:   ctx,
		Key:   key,
		Value: data,
		TTL:   ttl,
	})
}

func (m *MemoryCache) Get(ctx context.Context, key string, data interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
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
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cache.Delete(ctx, key)
}

func (m *MemoryCache) Consume(ctx context.Context, key string, data interface{}) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	err := m.cache.Get(ctx, key, &data)
	if errors.Is(err, cache.ErrCacheMiss) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := m.cache.Delete(ctx, key); err != nil {
		return false, err
	}
	return true, nil
}
