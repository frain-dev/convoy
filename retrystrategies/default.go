package retrystrategies

import (
	"time"
)

type DefaultRetryStrategy struct {
	intervalSeconds uint64
}

func (r *DefaultRetryStrategy) NextDuration(attempts uint64) time.Duration {
	return time.Duration(r.intervalSeconds) * time.Second
}

func NewDefault(intervalSeconds uint64) *DefaultRetryStrategy {
	return &DefaultRetryStrategy{
		intervalSeconds: intervalSeconds,
	}
}

var _ RetryStrategy = (*DefaultRetryStrategy)(nil)
