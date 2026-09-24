package retrystrategies

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/frain-dev/convoy/datastore"
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

func TestExponentialBackoffRetryStrategy_NextDuration_NearZeroDelays(t *testing.T) {
	m := datastore.Metadata{
		Strategy:        "exponential",
		RetryLimit:      20,
		IntervalSeconds: 0,
		MaxRetrySeconds: 14400,
	}
	r := NewRetryStrategyFromMetadata(m)
	_, isExp := r.(*ExponentialBackoffRetryStrategy)
	assert.True(t, isExp)

	for i := 0; i < 10; i++ {
		d := r.NextDuration(uint64(i))
		assert.GreaterOrEqual(t, d, 1*time.Second)
		assert.LessOrEqual(t, d, time.Duration(m.MaxRetrySeconds)*time.Second)
	}
}

func TestExponentialBackoffRetryStrategy_NextDuration_JitterNeverExceedsMax(t *testing.T) {
	m := datastore.Metadata{
		Strategy:        "exponential",
		RetryLimit:      20,
		IntervalSeconds: 2,
		MaxRetrySeconds: 120,
	}
	r := NewRetryStrategyFromMetadata(m)
	_, isExp := r.(*ExponentialBackoffRetryStrategy)
	assert.True(t, isExp)

	maxDuration := time.Duration(m.MaxRetrySeconds) * time.Second
	for i := 0; i < 1000; i++ {
		d := r.NextDuration(20)
		assert.LessOrEqual(t, d, maxDuration)
	}
}

func TestExponentialBackoffRetryStrategy_NextDuration_ExponentialGrowth(t *testing.T) {
	m := datastore.Metadata{
		Strategy:        "exponential",
		RetryLimit:      20,
		IntervalSeconds: 1,
		MaxRetrySeconds: 7200,
	}
	r := NewRetryStrategyFromMetadata(m)
	_, isExp := r.(*ExponentialBackoffRetryStrategy)
	assert.True(t, isExp)

	maxDuration := time.Duration(m.MaxRetrySeconds) * time.Second
	for i := 0; i < 12; i++ {
		d := r.NextDuration(uint64(i))

		assert.LessOrEqual(t, d, maxDuration)
		assert.Greater(t, d, time.Duration(0))
	}
}