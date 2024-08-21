package circuit_breaker

import (
	"context"
	"github.com/frain-dev/convoy/pkg/clock"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func getRedis(t *testing.T) (client redis.UniversalClient, err error) {
	t.Helper()

	opts, err := redis.ParseURL("redis://localhost:6379")
	if err != nil {
		return nil, err
	}

	return redis.NewClient(opts), nil
}

func TestNewCircuitBreaker(t *testing.T) {
	ctx := context.Background()

	re, err := getRedis(t)
	require.NoError(t, err)

	keys, err := re.Keys(ctx, "breaker*").Result()
	require.NoError(t, err)

	err = re.Del(ctx, keys...).Err()
	require.NoError(t, err)

	testClock := clock.NewSimulatedClock(time.Now())

	c := CircuitBreakerConfig{
		SampleTime:                  2,
		ErrorTimeout:                30,
		FailureThreshold:            10,
		FailureCount:                1,
		SuccessThreshold:            1,
		ObservabilityWindow:         5,
		NotificationThresholds:      []int{10},
		ConsecutiveFailureThreshold: 10,
	}
	b := NewCircuitBreakerManager(re, testClock, c)

	endpointId := "endpoint-1"
	pollResults := [][]PollResult{
		{
			PollResult{
				Key:       endpointId,
				Failures:  1,
				Successes: 0,
			},
		},
		{
			PollResult{
				Key:       endpointId,
				Failures:  1,
				Successes: 0,
			},
		},
		{
			PollResult{
				Key:       endpointId,
				Failures:  0,
				Successes: 1,
			},
		},
		{
			PollResult{
				Key:       endpointId,
				Failures:  0,
				Successes: 1,
			},
		},
	}

	for i := 0; i < len(pollResults); i++ {
		innerErr := b.sampleStore(ctx, pollResults[i])
		require.NoError(t, innerErr)

		testClock.AdvanceTime(time.Minute)
	}

	breaker, innerErr := b.GetCircuitBreaker(ctx, endpointId)
	require.NoError(t, innerErr)

	require.Equal(t, breaker.State, StateClosed)
}
