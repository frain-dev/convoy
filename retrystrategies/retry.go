package retrystrategies

import (
	"time"

	"github.com/frain-dev/convoy/datastore"
)

type RetryStrategy interface {
	// NextDuration is how long we should wait before next retry
	NextDuration(attempts uint64) time.Duration
}

func NewRetryStrategyFromMetadata(m datastore.Metadata) RetryStrategy {
	if string(m.Strategy) == string(datastore.ExponentialStrategyProvider) {
		// 10 seconds to 15 mins
		return NewExponential([]uint{
			10000,  // 10 seconds
			30000,  // 30 seconds
			60000,  // 1 minute
			180000, // 3 minutes
			300000, // 5 minutes
			600000, // 10 minutes
			900000, // 15 minutes
		})
	}

	return NewDefault(m.IntervalSeconds)
}
