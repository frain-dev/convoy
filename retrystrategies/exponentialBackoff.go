package retrystrategies

import (
	"math"
	"math/rand"
	"time"
)

type JitterFn func(uint) int

type ExponentialBackoffRetryStrategy struct {
	intervalSeconds uint64
	retryLimit      uint64
}

func (r *ExponentialBackoffRetryStrategy) NextDuration(attempts uint64) time.Duration {

	retrySeconds := float64(r.intervalSeconds) * math.Pow(2, float64(attempts%18)) // reset after 18 attempts

	d := time.Duration(retrySeconds) * time.Second

	jitter := time.Duration(rand.Uint64() % 10e9)

	d += jitter / 2
	return d
}

func NewExponential(intervalSeconds uint64, retryLimit uint64) *ExponentialBackoffRetryStrategy {
	return &ExponentialBackoffRetryStrategy{
		intervalSeconds: intervalSeconds,
		retryLimit:      retryLimit,
	}
}

var _ RetryStrategy = (*ExponentialBackoffRetryStrategy)(nil)
