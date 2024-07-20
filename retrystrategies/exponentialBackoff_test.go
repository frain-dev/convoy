package retrystrategies

import (
	"fmt"
	"github.com/frain-dev/convoy/datastore"
	"github.com/stretchr/testify/assert"
	"math"
	"testing"
	"time"
)

func TestExponentialBackoffRetryStrategy_NextDuration(t *testing.T) {
	m := datastore.Metadata{
		Strategy:        "exponential",
		RetryLimit:      20,
		IntervalSeconds: 100,
	}
	var r = NewRetryStrategyFromMetadata(m)
	_, isExp := r.(*ExponentialBackoffRetryStrategy)
	assert.True(t, isExp)

	for i := 0; i < 100; i++ {
		d := r.NextDuration(uint64(i))
		expected := time.Duration(float64(m.IntervalSeconds)*math.Pow(2, float64(i))) * time.Second
		fmt.Println("i: " + fmt.Sprint(i) + " diff: " + expected.String() + " d: " + d.String())
		assert.True(t, d < 3*time.Hour)
	}
}

func TestExponentialBackoffRetryStrategy_NextDuration_MaxFourHours(t *testing.T) {
	m := datastore.Metadata{
		Strategy:        "exponential",
		RetryLimit:      20,
		IntervalSeconds: 2,
		MaxRetrySeconds: 14400,
	}
	var r = NewRetryStrategyFromMetadata(m)
	_, isExp := r.(*ExponentialBackoffRetryStrategy)
	assert.True(t, isExp)

	for i := 0; i < 100; i++ {
		d := r.NextDuration(uint64(i))
		expected := time.Duration(float64(m.IntervalSeconds)*math.Pow(2, float64(i))) * time.Second
		fmt.Println("i: " + fmt.Sprint(i) + " diff: " + expected.String() + " d: " + d.String())
		assert.True(t, d < 5*time.Hour)
	}
}
