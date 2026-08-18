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
	// GetBytes reads a raw byte value written by GetOrCreateBytes. Miss returns nil, nil.
	GetBytes(ctx context.Context, key string) ([]byte, error)
	GetOrCreateBytes(ctx context.Context, key string, value []byte) ([]byte, error)
}
