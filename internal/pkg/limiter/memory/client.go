package mlimiter

import (
	"context"
	"go.uber.org/ratelimit"
)

type MemoryRateLimiter struct {
	limiters map[string]ratelimit.Limiter
}

// NewMemoryRateLimiter creates a new instance of MemoryRateLimiter.
func NewMemoryRateLimiter(keys []string, rate int) *MemoryRateLimiter {
	m := make(map[string]ratelimit.Limiter, len(keys))
	for i := 0; i < len(keys); i++ {
		m[keys[i]] = ratelimit.New(rate)
	}

	return &MemoryRateLimiter{
		limiters: m,
	}
}

// Allow blocks till the window has completed then takes a token
func (r *MemoryRateLimiter) Allow(_ context.Context, key string, _, _ int) error {
	limiter := r.limiters[key]
	limiter.Take()

	return nil
}
