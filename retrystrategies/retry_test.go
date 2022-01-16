package retrystrategies

import (
	"testing"

	"github.com/frain-dev/convoy/datastore"
	"github.com/stretchr/testify/assert"
)

func TestRetry_CreatesExponential(t *testing.T) {
	m := datastore.Metadata{
		Strategy:        "exponential-backoff",
		RetryLimit:      20,
		IntervalSeconds: 5,
	}
	var r RetryStrategy = NewRetryStrategyFromMetadata(m)
	assert.NotEqual(t, r.NextDuration(1), r.NextDuration(3))
}

func TestRetry_CreatesDefault(t *testing.T) {
	m := datastore.Metadata{
		Strategy:        "default",
		RetryLimit:      20,
		IntervalSeconds: 5,
	}

	var r RetryStrategy = NewRetryStrategyFromMetadata(m)
	assert.Equal(t, r.NextDuration(1), r.NextDuration(3))
}

func TestRetry_FallsBackToDefault(t *testing.T) {
	m := datastore.Metadata{
		Strategy:        "",
		RetryLimit:      20,
		IntervalSeconds: 5,
	}
	var r RetryStrategy = NewRetryStrategyFromMetadata(m)
	assert.Equal(t, r.NextDuration(1), r.NextDuration(3))
}
