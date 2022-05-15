package ncache

import (
	"context"
	"time"
)

type NoopCache struct{}

func NewNoopCache() *NoopCache {
	return &NoopCache{}
}

func (n *NoopCache) Set(ctx context.Context, key string, data interface{}, ttl time.Duration) error {
	return nil
}

func (n *NoopCache) Get(ctx context.Context, key string, data interface{}) error {
	return nil
}

func (n *NoopCache) Delete(ctx context.Context, key string) error {
	return nil
}
