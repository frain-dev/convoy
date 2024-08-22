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

func pollResult(t *testing.T, key string, failureCount, successCount uint64) PollResult {
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

	testClock := clock.NewSimulatedClock(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))

	c := &CircuitBreakerConfig{
		SampleRate:                  2,
		ErrorTimeout:                30,
		FailureThreshold:            0.1,
		FailureCount:                3,
		SuccessThreshold:            1,
		ObservabilityWindow:         5,
		NotificationThresholds:      []uint64{10},
		ConsecutiveFailureThreshold: 10,
	}

	b, err := NewCircuitBreakerManager(re).WithClock(testClock)
	require.NoError(t, err)

	b, err = b.WithConfig(c)
	require.NoError(t, err)

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

func TestNewCircuitBreaker_AddNewBreakerMidway(t *testing.T) {
	ctx := context.Background()

	re, err := getRedis(t)
	require.NoError(t, err)

	keys, err := re.Keys(ctx, "breaker*").Result()
	require.NoError(t, err)

	err = re.Del(ctx, keys...).Err()
	require.NoError(t, err)

	testClock := clock.NewSimulatedClock(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))

	c := &CircuitBreakerConfig{
		SampleRate:                  2,
		ErrorTimeout:                30,
		FailureThreshold:            0.1,
		FailureCount:                3,
		SuccessThreshold:            1,
		ObservabilityWindow:         5,
		NotificationThresholds:      []uint64{10},
		ConsecutiveFailureThreshold: 10,
	}
	b, err := NewCircuitBreakerManager(re).WithClock(testClock)
	require.NoError(t, err)

	b, err = b.WithConfig(c)
	require.NoError(t, err)

	endpoint1 := "endpoint-1"
	endpoint2 := "endpoint-2"
	pollResults := [][]PollResult{
		{
			pollResult(t, endpoint1, 1, 0),
		},
		{
			pollResult(t, endpoint1, 2, 0),
		},
		{
			pollResult(t, endpoint1, 2, 1),
			pollResult(t, endpoint2, 1, 0),
		},
		{
			pollResult(t, endpoint1, 2, 2),
			pollResult(t, endpoint2, 1, 1),
		},
		{
			pollResult(t, endpoint1, 2, 3),
			pollResult(t, endpoint2, 0, 2),
		},
		{
			pollResult(t, endpoint1, 1, 4),
			pollResult(t, endpoint2, 1, 1),
		},
	}

	for i := 0; i < len(pollResults); i++ {
		err = b.sampleStore(ctx, pollResults[i])
		require.NoError(t, err)

		if i > 1 {
			breaker, innerErr := b.GetCircuitBreaker(ctx, endpoint2)
			require.NoError(t, innerErr)
			t.Logf("%+v\n", breaker)
		}

		testClock.AdvanceTime(time.Minute)
	}

	breakers, innerErr := b.loadCircuitBreakers(ctx)
	require.NoError(t, innerErr)

	require.Len(t, breakers, 2)
}
