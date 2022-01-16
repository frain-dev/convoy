package retrystrategies

import (
	"math/rand"
	"time"
)

// based off https://blog.gopheracademy.com/advent-2014/backoff/
type ExponentialBackoffRetryStrategy struct {
	millis []uint
}

func (r *ExponentialBackoffRetryStrategy) NextDuration(attempts uint64) time.Duration {
	if int(attempts) >= len(r.millis) {
		attempts = uint64(len(r.millis) - 1)
	}

	return time.Duration(jitter(r.millis[attempts])) * time.Millisecond
}

func jitter(millis uint) int {
	if millis == 0 {
		return 0
	}

	return int(millis/2) + rand.Intn(int(millis))
}

func NewExponential(millis []uint) *ExponentialBackoffRetryStrategy {
	return &ExponentialBackoffRetryStrategy{
		millis: millis,
	}
}

var _ RetryStrategy = (*ExponentialBackoffRetryStrategy)(nil)
