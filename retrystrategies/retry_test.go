package retrystrategies

import (
	"testing"

	"github.com/frain-dev/convoy/datastore"
	"github.com/stretchr/testify/assert"
)

func TestRetry_CreatesExponential(t *testing.T) {
	m := datastore.Metadata{
		Strategy:        "exponential",
		RetryLimit:      20,
		IntervalSeconds: 5,
	}
	var r RetryStrategy = NewRetryStrategyFromMetadata(m)
	_, isExponential := r.(*ExponentialBackoffRetryStrategy)
	assert.True(t, isExponential)
}

func TestRetry_CreatesLinear(t *testing.T) {
	m := datastore.Metadata{
		Strategy:        "linear",
		RetryLimit:      20,
		IntervalSeconds: 5,
	}

	var r RetryStrategy = NewRetryStrategyFromMetadata(m)
	_, isDefault := r.(*DefaultRetryStrategy)
	assert.True(t, isDefault)
}

func TestRetry_FallsBackToDefault(t *testing.T) {
	m := datastore.Metadata{
		Strategy:        "",
		RetryLimit:      20,
		IntervalSeconds: 5,
	}
	var r RetryStrategy = NewRetryStrategyFromMetadata(m)
	_, isDefault := r.(*DefaultRetryStrategy)
	assert.True(t, isDefault)
}
