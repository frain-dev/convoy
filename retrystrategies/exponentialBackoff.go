package retrystrategies

import (
	"math"
	"math/rand"
	"time"
)

type JitterFn func(uint) int

type ExponentialBackoffRetryStrategy struct {
	intervalSeconds uint64
	maxRetrySeconds uint64
}

func (r *ExponentialBackoffRetryStrategy) NextDuration(attempts uint64) time.Duration {
	retrySeconds := float64(r.intervalSeconds) * math.Pow(2, float64(attempts))
	if uint64(retrySeconds) > r.maxRetrySeconds {
		retrySeconds = float64(r.maxRetrySeconds)
	}

	d := time.Duration(retrySeconds) * time.Second

	jitter := time.Duration(rand.Uint64() % 10e9)

	d += jitter / 2

	return d
}

func NewExponential(intervalSeconds uint64, maxRetrySeconds uint64) *ExponentialBackoffRetryStrategy {
	if maxRetrySeconds == 0 {
		maxRetrySeconds = 7200
	}

	return &ExponentialBackoffRetryStrategy{
		intervalSeconds: intervalSeconds,
		maxRetrySeconds: maxRetrySeconds,
	}
}

var _ RetryStrategy = (*ExponentialBackoffRetryStrategy)(nil)
