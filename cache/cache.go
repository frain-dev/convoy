package cache

import (
	"context"
	"time"
)

type Cache interface {
	Set(ctx context.Context, key string, data interface{}, expiration time.Duration) error
	Get(ctx context.Context, key string, data interface{}) error
	Delete(ctx context.Context, key string) error
}

type AuthoritativeCache interface {
	Cache
	GetStrict(ctx context.Context, key string, data interface{}) error
	GetOrCreateBytes(ctx context.Context, key string, value []byte) ([]byte, error)
}
