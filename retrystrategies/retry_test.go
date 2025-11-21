package retrystrategies

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/frain-dev/convoy/datastore"
)

func TestRetry_CreatesExponential(t *testing.T) {
	m := datastore.Metadata{
		Strategy:        "exponential",
		RetryLimit:      20,
		IntervalSeconds: 5,
	}
	r := NewRetryStrategyFromMetadata(m)
	_, isExponential := r.(*ExponentialBackoffRetryStrategy)
	assert.True(t, isExponential)
}

func TestRetry_CreatesLinear(t *testing.T) {
	m := datastore.Metadata{
		Strategy:        "linear",
		RetryLimit:      20,
		IntervalSeconds: 5,
	}

	r := NewRetryStrategyFromMetadata(m)
	_, isDefault := r.(*DefaultRetryStrategy)
	assert.True(t, isDefault)
}

func TestRetry_FallsBackToDefault(t *testing.T) {
	m := datastore.Metadata{
		Strategy:        "",
		RetryLimit:      20,
		IntervalSeconds: 5,
	}
	r := NewRetryStrategyFromMetadata(m)
	_, isDefault := r.(*DefaultRetryStrategy)
	assert.True(t, isDefault)
}
