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

func pollResult(t *testing.T, key string, failureCount, successCount int) PollResult {
	t.Helper()

	return PollResult{
		Key:       key,
		Failures:  failureCount,
		Successes: successCount,
	}
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

	c := &CircuitBreakerConfig{
		SampleRate:                  2,
		ErrorTimeout:                30,
		FailureThreshold:            0.1,
		FailureCount:                3,
		SuccessThreshold:            1,
		ObservabilityWindow:         5,
		NotificationThresholds:      []int{10},
		ConsecutiveFailureThreshold: 10,
	}
	b := NewCircuitBreakerManager(re).WithClock(testClock).WithConfig(c)

	endpointId := "endpoint-1"
	pollResults := [][]PollResult{
		{
			pollResult(t, endpointId, 1, 0),
		},
		{
			pollResult(t, endpointId, 2, 0),
		},
		{
			pollResult(t, endpointId, 2, 1),
		},
		{
			pollResult(t, endpointId, 2, 2),
		},
		{
			pollResult(t, endpointId, 2, 3),
		},
		{
			pollResult(t, endpointId, 1, 4),
		},
		{
			pollResult(t, endpointId, 0, 5),
		},
	}

	for i := 0; i < len(pollResults); i++ {
		innerErr := b.sampleStore(ctx, pollResults[i])
		require.NoError(t, innerErr)

		breaker, innerErr := b.GetCircuitBreaker(ctx, endpointId)
		require.NoError(t, innerErr)
		t.Logf("%+v\n", breaker)

		testClock.AdvanceTime(time.Minute)
	}

	breaker, innerErr := b.GetCircuitBreaker(ctx, endpointId)
	require.NoError(t, innerErr)

	require.Equal(t, breaker.State, StateClosed)
}
