package limiter

import (
	"context"
	"errors"
	"time"
)

var ErrRateLimitExceeded = errors.New("rate limit exceeded")

type RateLimiter interface {
	// Allow rate limits outgoing events to endpoints based on a rate in a specified time duration by the endpoint id
	Allow(ctx context.Context, key string, rate int) error
	AllowWithDuration(ctx context.Context, key string, rate int, duration int) error
}

type RateLimitError struct {
	delay time.Duration
	err   error
}

func NewRateLimitExceeded(delay time.Duration) *RateLimitError {
	return &RateLimitError{delay: delay, err: ErrRateLimitExceeded}
}

func (e *RateLimitError) Error() string {
	return e.err.Error()
}

func GetRetryAfter(err error) time.Duration {
	if rateLimitError, ok := err.(*RateLimitError); ok {
		return rateLimitError.delay
	}
	return 0
}

func GetRawError(err error) error {
	if rateLimitError, ok := err.(*RateLimitError); ok {
		return rateLimitError.err
	}
	return nil
}
