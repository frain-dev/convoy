package retrystrategies

import (
	"time"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
)

type RetryStrategy interface {
	// NextDuration is how long we should wait before next retry
	NextDuration(attempts uint64) time.Duration
}

func NewRetryStrategyFromMetadata(m datastore.Metadata) RetryStrategy {
	if string(m.Strategy) == string(config.ExponentialBackoffStrategyProvider) {
		// 0 to 5 seconds
		return NewExponential([]uint{0, 10, 10, 100, 100, 500, 500, 3000, 3000, 5000})
	}

	return NewDefault(m.IntervalSeconds)
}
