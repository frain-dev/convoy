// Package cachedrepo provides generic cache-aside helpers for repository patterns.
// It has no external dependencies and can be imported in any Go project.
package cachedrepo

import (
	"context"
	"time"
)

// Cache is a minimal cache interface.
// Get must return nil (not an error) on cache miss.
type Cache interface {
	Set(ctx context.Context, key string, data interface{}, ttl time.Duration) error
	Get(ctx context.Context, key string, data interface{}) error
	Delete(ctx context.Context, key string) error
}

// Logger is a minimal structured logger interface.
type Logger interface {
	Error(args ...any)
}

// sliceWrapper distinguishes "empty result cached" from "cache miss" for slice types.
type sliceWrapper[T any] struct {
	Items []T
}

// foundWrapper distinguishes "cache miss" from "cached not-found" for single-entity lookups.
type foundWrapper[T any] struct {
	Value *T
	Found bool
}

// FetchOne performs read-through caching for single-entity lookups.
// hitCheck returns true if the cached value is populated (e.g., entity.UID != "").
// On cache miss, calls fetch, caches the result, and returns it.
// Cache errors are logged but never propagated.
func FetchOne[T any](
	ctx context.Context, ca Cache, logger Logger,
	key string, ttl time.Duration,
	hitCheck func(*T) bool,
	fetch func() (*T, error),
) (*T, error) {
	var result T
	err := ca.Get(ctx, key, &result)
	if err != nil {
		logger.Error("cache get error", "key", key, "error", err)
	}

	if hitCheck(&result) {
		return &result, nil
	}

	val, err := fetch()
	if err != nil {
		return nil, err
	}

	if setErr := ca.Set(ctx, key, val, ttl); setErr != nil {
		logger.Error("cache set error", "key", key, "error", setErr)
	}

	return val, nil
}

// FetchSlice performs read-through caching for slice lookups.
// Internally wraps the slice to distinguish "empty result cached" from "cache miss".
// Cache errors are logged but never propagated.
func FetchSlice[T any](
	ctx context.Context, ca Cache, logger Logger,
	key string, ttl time.Duration,
	fetch func() ([]T, error),
) ([]T, error) {
	var cached sliceWrapper[T]
	err := ca.Get(ctx, key, &cached)
	if err != nil {
		logger.Error("cache get error", "key", key, "error", err)
	}

	if cached.Items != nil {
		return cached.Items, nil
	}

	items, err := fetch()
	if err != nil {
		return nil, err
	}

	toCache := sliceWrapper[T]{Items: items}
	if setErr := ca.Set(ctx, key, &toCache, ttl); setErr != nil {
		logger.Error("cache set error", "key", key, "error", setErr)
	}

	return items, nil
}

// FetchWithNotFound is like FetchOne but also caches not-found results.
// When fetch returns an error where isNotFound(err) is true, the not-found
// is cached so subsequent calls skip the DB. Returns (nil, original error) on not-found.
// Cache errors are logged but never propagated.
func FetchWithNotFound[T any](
	ctx context.Context, ca Cache, logger Logger,
	key string, ttl time.Duration,
	fetch func() (*T, error),
	isNotFound func(error) bool,
) (*T, error) {
	var cached foundWrapper[T]
	err := ca.Get(ctx, key, &cached)
	if err != nil {
		logger.Error("cache get error", "key", key, "error", err)
	}

	if cached.Found {
		return cached.Value, nil
	}

	val, err := fetch()
	if err != nil {
		if isNotFound(err) {
			toCache := foundWrapper[T]{Value: nil, Found: true}
			if setErr := ca.Set(ctx, key, &toCache, ttl); setErr != nil {
				logger.Error("cache set error", "key", key, "error", setErr)
			}
		}
		return nil, err
	}

	toCache := foundWrapper[T]{Value: val, Found: true}
	if setErr := ca.Set(ctx, key, &toCache, ttl); setErr != nil {
		logger.Error("cache set error", "key", key, "error", setErr)
	}

	return val, nil
}

// Invalidate deletes one or more cache keys. Empty keys are skipped.
// Errors are logged but never propagated.
func Invalidate(ctx context.Context, ca Cache, logger Logger, keys ...string) {
	for _, key := range keys {
		if key == "" {
			continue
		}
		if err := ca.Delete(ctx, key); err != nil {
			logger.Error("cache delete error", "key", key, "error", err)
		}
	}
}
