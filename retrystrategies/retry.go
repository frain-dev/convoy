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
		return NewExponential(m.IntervalSeconds, m.RetryLimit)
	}

	return NewDefault(m.IntervalSeconds)
}
