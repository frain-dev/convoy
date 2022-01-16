package retrystrategies

import (
	"math/rand"
	"time"
)

type JitterFn func(uint) int

// based off https://blog.gopheracademy.com/advent-2014/backoff/
type ExponentialBackoffRetryStrategy struct {
	millis   []uint
	jitterFn JitterFn
}

func (r *ExponentialBackoffRetryStrategy) NextDuration(attempts uint64) time.Duration {
	if int(attempts) >= len(r.millis) {
		attempts = uint64(len(r.millis) - 1)
	}

	return time.Duration(r.jitterFn(r.millis[attempts])) * time.Millisecond
}

func jitter(millis uint) int {
	if millis == 0 {
		return 0
	}

	return int(millis/2) + rand.Intn(int(millis))
}

func NewExponential(millis []uint) *ExponentialBackoffRetryStrategy {
	return &ExponentialBackoffRetryStrategy{
		millis:   millis,
		jitterFn: jitter,
	}
}

func NewExponentialWithJitter(millis []uint, customJitter JitterFn) *ExponentialBackoffRetryStrategy {
	return &ExponentialBackoffRetryStrategy{
		millis:   millis,
		jitterFn: customJitter,
	}
}

var _ RetryStrategy = (*ExponentialBackoffRetryStrategy)(nil)
