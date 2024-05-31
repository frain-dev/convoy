package mlimiter

import (
	"context"
	"go.uber.org/ratelimit"
	"sync"
)

type MemoryRateLimiter struct {
	mu       sync.Mutex
	limiters map[string]ratelimit.Limiter
}

// NewMemoryRateLimiter creates a new instance of MemoryRateLimiter.
func NewMemoryRateLimiter() *MemoryRateLimiter {
	return &MemoryRateLimiter{
		limiters: make(map[string]ratelimit.Limiter),
	}
}

// Allow blocks till the window has completed then takes a token
func (r *MemoryRateLimiter) Allow(_ context.Context, key string, rate, _ int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	limiter, exists := r.limiters[key]
	if !exists {
		limiter = ratelimit.New(rate)
		r.limiters[key] = limiter
	}

	limiter.Take()

	return nil
}
