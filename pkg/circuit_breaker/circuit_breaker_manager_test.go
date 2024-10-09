package circuit_breaker

import (
	"context"
	"errors"
	"github.com/frain-dev/convoy/pkg/log"
	"os"
	"testing"
	"time"

	"github.com/frain-dev/convoy/pkg/clock"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func getRedis(t *testing.T) (client redis.UniversalClient, err error) {
	t.Helper()

	opts, err := redis.ParseURL("redis://localhost:6379")
	if err != nil {
		return nil, err
	}

	return redis.NewClient(opts), nil
}

func pollResult(t *testing.T, key string, failureCount, successCount uint64) map[string]PollResult {
	t.Helper()

	return map[string]PollResult{
		key: {
			Key:       key,
			Failures:  failureCount,
			Successes: successCount,
		},
	}
}

func TestCircuitBreakerManager(t *testing.T) {
	ctx := context.Background()

	testClock := clock.NewSimulatedClock(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))

	re, err := getRedis(t)
	require.NoError(t, err)

	store := NewRedisStore(re, testClock)

	keys, err := re.Keys(ctx, "breaker*").Result()
	require.NoError(t, err)

	for i := range keys {
		err = re.Del(ctx, keys[i]).Err()
		require.NoError(t, err)
	}

	c := &CircuitBreakerConfig{
		SampleRate:                  2,
		BreakerTimeout:              30,
		FailureThreshold:            70,
		SuccessThreshold:            10,
		MinimumRequestCount:         10,
		ObservabilityWindow:         5,
		ConsecutiveFailureThreshold: 10,
	}

	b, err := NewCircuitBreakerManager(
		ClockOption(testClock),
		StoreOption(store),
		ConfigOption(c),
		LoggerOption(log.NewLogger(os.Stdout)),
	)
	require.NoError(t, err)

	endpointId := "endpoint-1"
	pollResults := []map[string]PollResult{
		pollResult(t, endpointId, 3, 9),
		pollResult(t, endpointId, 5, 10),
		pollResult(t, endpointId, 10, 1),
		pollResult(t, endpointId, 20, 0),
		pollResult(t, endpointId, 2, 3),
		pollResult(t, endpointId, 1, 4),
	}

	for i := 0; i < len(pollResults); i++ {
		innerErr := b.sampleStore(ctx, pollResults[i])
		require.NoError(t, innerErr)

		testClock.AdvanceTime(time.Minute)
	}

	breaker, innerErr := b.GetCircuitBreakerWithError(ctx, endpointId)
	require.NoError(t, innerErr)

	require.Equal(t, breaker.State, StateClosed)
}

func TestCircuitBreakerManager_AddNewBreakerMidway(t *testing.T) {
	ctx := context.Background()

	testClock := clock.NewSimulatedClock(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))

	re, err := getRedis(t)
	require.NoError(t, err)

	store := NewRedisStore(re, testClock)

	keys, err := re.Keys(ctx, "breaker*").Result()
	require.NoError(t, err)

	for i := range keys {
		err = re.Del(ctx, keys[i]).Err()
		require.NoError(t, err)
	}

	c := &CircuitBreakerConfig{
		SampleRate:                  2,
		BreakerTimeout:              30,
		FailureThreshold:            70,
		SuccessThreshold:            10,
		MinimumRequestCount:         10,
		ObservabilityWindow:         5,
		ConsecutiveFailureThreshold: 10,
	}
	b, err := NewCircuitBreakerManager(ClockOption(testClock), StoreOption(store), ConfigOption(c), LoggerOption(log.NewLogger(os.Stdout)))
	require.NoError(t, err)

	endpoint1 := "endpoint-1"
	endpoint2 := "endpoint-2"
	pollResults := []map[string]PollResult{
		pollResult(t, endpoint1, 1, 0),
		pollResult(t, endpoint1, 2, 0),
		pollResult(t, endpoint1, 2, 1), pollResult(t, endpoint2, 1, 0),
		pollResult(t, endpoint1, 2, 2), pollResult(t, endpoint2, 1, 1),
		pollResult(t, endpoint1, 2, 3), pollResult(t, endpoint2, 0, 2),
		pollResult(t, endpoint1, 1, 4), pollResult(t, endpoint2, 1, 1),
	}

	for i := 0; i < len(pollResults); i++ {
		err = b.sampleStore(ctx, pollResults[i])
		require.NoError(t, err)

		testClock.AdvanceTime(time.Minute)
	}

	breakers, innerErr := b.loadCircuitBreakers(ctx)
	require.NoError(t, innerErr)

	require.Len(t, breakers, 2)
}

func TestCircuitBreakerManager_Transitions(t *testing.T) {
	ctx := context.Background()

	testClock := clock.NewSimulatedClock(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))

	re, err := getRedis(t)
	require.NoError(t, err)

	store := NewRedisStore(re, testClock)

	keys, err := store.Keys(ctx, "breaker*")
	require.NoError(t, err)

	for i := range keys {
		err = re.Del(ctx, keys[i]).Err()
		require.NoError(t, err)
	}

	c := &CircuitBreakerConfig{
		SampleRate:                  2,
		BreakerTimeout:              30,
		FailureThreshold:            50,
		SuccessThreshold:            10,
		MinimumRequestCount:         10,
		ObservabilityWindow:         5,
		ConsecutiveFailureThreshold: 10,
	}
	b, err := NewCircuitBreakerManager(ClockOption(testClock), StoreOption(store), ConfigOption(c), LoggerOption(log.NewLogger(os.Stdout)))
	require.NoError(t, err)

	endpointId := "endpoint-1"
	pollResults := []map[string]PollResult{
		pollResult(t, endpointId, 1, 2),  // Closed
		pollResult(t, endpointId, 13, 1), // Still Open
		pollResult(t, endpointId, 10, 1), // Half-Open (after ErrorTimeout)
		pollResult(t, endpointId, 0, 2),  // Closed (SuccessThreshold reached)
		pollResult(t, endpointId, 14, 0), // Open (FailureThreshold reached)
	}

	expectedStates := []State{
		StateClosed,
		StateOpen,
		StateHalfOpen,
		StateClosed,
		StateOpen,
	}

	for i, result := range pollResults {
		err = b.sampleStore(ctx, result)
		require.NoError(t, err)

		breaker, innerErr := b.GetCircuitBreakerWithError(ctx, endpointId)
		require.NoError(t, innerErr)

		require.Equal(t, expectedStates[i], breaker.State, "Iteration %d: expected state %v, got %v", i, expectedStates[i], breaker.State)

		if i == 1 {
			// Advance time to trigger the transition to half-open
			testClock.AdvanceTime(time.Duration(c.BreakerTimeout+1) * time.Second)
		} else {
			testClock.AdvanceTime(time.Second * 5) // Advance time by 5 seconds for other iterations
		}
	}
}

func TestCircuitBreakerManager_ConsecutiveFailures(t *testing.T) {
	ctx := context.Background()

	testClock := clock.NewSimulatedClock(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))

	re, err := getRedis(t)
	require.NoError(t, err)

	store := NewRedisStore(re, testClock)

	keys, err := re.Keys(ctx, "breaker*").Result()
	require.NoError(t, err)

	for i := range keys {
		err = re.Del(ctx, keys[i]).Err()
		require.NoError(t, err)
	}

	c := &CircuitBreakerConfig{
		SampleRate:                  2,
		BreakerTimeout:              30,
		FailureThreshold:            70,
		SuccessThreshold:            10,
		MinimumRequestCount:         10,
		ObservabilityWindow:         5,
		ConsecutiveFailureThreshold: 3,
	}
	b, err := NewCircuitBreakerManager(ClockOption(testClock), StoreOption(store), ConfigOption(c), LoggerOption(log.NewLogger(os.Stdout)))
	require.NoError(t, err)

	endpointId := "endpoint-1"
	pollResults := []map[string]PollResult{
		pollResult(t, endpointId, 13, 1), // Open
		pollResult(t, endpointId, 13, 1), // Half-Open
		pollResult(t, endpointId, 15, 0), // Open
		pollResult(t, endpointId, 17, 1), // Half-Open
		pollResult(t, endpointId, 13, 0), // Open
	}

	for _, result := range pollResults {
		err = b.sampleStore(ctx, result)
		require.NoError(t, err)

		testClock.AdvanceTime(time.Duration(c.BreakerTimeout+1) * time.Second)
	}

	breaker, err := b.GetCircuitBreakerWithError(ctx, endpointId)
	require.NoError(t, err)
	require.Equal(t, StateOpen, breaker.State)
	require.Equal(t, uint64(3), breaker.ConsecutiveFailures)
}

func TestCircuitBreakerManager_MultipleEndpoints(t *testing.T) {
	ctx := context.Background()

	testClock := clock.NewSimulatedClock(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))

	re, err := getRedis(t)
	require.NoError(t, err)

	store := NewRedisStore(re, testClock)

	keys, err := re.Keys(ctx, "breaker*").Result()
	require.NoError(t, err)

	for i := range keys {
		err = re.Del(ctx, keys[i]).Err()
		require.NoError(t, err)
	}

	c := &CircuitBreakerConfig{
		SampleRate:                  2,
		BreakerTimeout:              30,
		FailureThreshold:            60,
		SuccessThreshold:            10,
		ObservabilityWindow:         5,
		MinimumRequestCount:         10,
		ConsecutiveFailureThreshold: 10,
	}
	b, err := NewCircuitBreakerManager(ClockOption(testClock), StoreOption(store), ConfigOption(c), LoggerOption(log.NewLogger(os.Stdout)))
	require.NoError(t, err)

	endpoint1 := "endpoint-1"
	endpoint2 := "endpoint-2"
	endpoint3 := "endpoint-3"

	pollResults := []map[string]PollResult{
		pollResult(t, endpoint1, 10, 0), pollResult(t, endpoint2, 3, 1), pollResult(t, endpoint3, 0, 4),
		pollResult(t, endpoint1, 13, 0), pollResult(t, endpoint2, 3, 1), pollResult(t, endpoint3, 0, 4),
		pollResult(t, endpoint1, 15, 0), pollResult(t, endpoint2, 1, 3), pollResult(t, endpoint3, 1, 5),
	}

	for _, results := range pollResults {
		err = b.sampleStore(ctx, results)
		require.NoError(t, err)

		testClock.AdvanceTime(time.Duration(c.BreakerTimeout+1) * time.Second)
	}

	breaker1, err := b.GetCircuitBreakerWithError(ctx, endpoint1)
	require.NoError(t, err)
	require.Equal(t, StateOpen, breaker1.State)

	breaker2, err := b.GetCircuitBreakerWithError(ctx, endpoint2)
	require.NoError(t, err)
	require.Equal(t, StateClosed, breaker2.State)

	breaker3, err := b.GetCircuitBreakerWithError(ctx, endpoint3)
	require.NoError(t, err)
	require.Equal(t, StateClosed, breaker3.State)
}

func TestCircuitBreakerManager_Config(t *testing.T) {
	mockStore := NewTestStore()
	mockClock := clock.NewSimulatedClock(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	config := &CircuitBreakerConfig{
		SampleRate:                  1,
		BreakerTimeout:              30,
		FailureThreshold:            50,
		SuccessThreshold:            10,
		MinimumRequestCount:         10,
		ObservabilityWindow:         5,
		ConsecutiveFailureThreshold: 3,
	}

	t.Run("Success", func(t *testing.T) {
		manager, err := NewCircuitBreakerManager(
			StoreOption(mockStore),
			ClockOption(mockClock),
			ConfigOption(config),
			LoggerOption(log.NewLogger(os.Stdout)),
		)

		require.NoError(t, err)
		require.NotNil(t, manager)
		require.Equal(t, mockStore, manager.store)
		require.Equal(t, mockClock, manager.clock)
		require.Equal(t, config, manager.config)
	})

	t.Run("Missing Store", func(t *testing.T) {
		_, err := NewCircuitBreakerManager(
			ClockOption(mockClock),
			ConfigOption(config),
			LoggerOption(log.NewLogger(os.Stdout)),
		)

		require.Error(t, err)
		require.Equal(t, ErrStoreMustNotBeNil, err)
	})

	t.Run("Missing Clock", func(t *testing.T) {
		_, err := NewCircuitBreakerManager(
			StoreOption(mockStore),
			ConfigOption(config),
			LoggerOption(log.NewLogger(os.Stdout)),
		)

		require.Error(t, err)
		require.Equal(t, ErrClockMustNotBeNil, err)
	})

	t.Run("Missing Config", func(t *testing.T) {
		_, err := NewCircuitBreakerManager(
			StoreOption(mockStore),
			ClockOption(mockClock),
			LoggerOption(log.NewLogger(os.Stdout)),
		)

		require.Error(t, err)
		require.Equal(t, ErrConfigMustNotBeNil, err)
	})
}

func TestCircuitBreakerManager_GetCircuitBreakerError(t *testing.T) {
	config := &CircuitBreakerConfig{
		SampleRate:                  1,
		BreakerTimeout:              30,
		FailureThreshold:            50,
		SuccessThreshold:            10,
		MinimumRequestCount:         10,
		ObservabilityWindow:         5,
		ConsecutiveFailureThreshold: 3,
	}

	c := clock.NewSimulatedClock(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	manager := &CircuitBreakerManager{config: config, clock: c}

	t.Run("Open State", func(t *testing.T) {
		breaker := CircuitBreaker{State: StateOpen}
		err := manager.getCircuitBreakerError(breaker)
		require.Equal(t, ErrOpenState, err)
	})

	t.Run("Half-Open State with Too Many Failures", func(t *testing.T) {
		breaker := CircuitBreaker{State: StateHalfOpen, FailureRate: 60, WillResetAt: time.Date(2020, 1, 1, 0, 1, 0, 0, time.UTC)}
		err := manager.getCircuitBreakerError(breaker)
		require.Equal(t, ErrTooManyRequests, err)
	})

	t.Run("Half-Open State with Acceptable Failures", func(t *testing.T) {
		breaker := CircuitBreaker{State: StateHalfOpen, FailureRate: 40}
		err := manager.getCircuitBreakerError(breaker)
		require.NoError(t, err)
	})

	t.Run("Closed State", func(t *testing.T) {
		breaker := CircuitBreaker{State: StateClosed}
		err := manager.getCircuitBreakerError(breaker)
		require.NoError(t, err)
	})
}

func TestCircuitBreakerManager_SampleStore(t *testing.T) {
	mockStore := NewTestStore()
	mockClock := clock.NewSimulatedClock(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	config := &CircuitBreakerConfig{
		SampleRate:                  1,
		BreakerTimeout:              30,
		FailureThreshold:            50,
		SuccessThreshold:            10,
		MinimumRequestCount:         10,
		ObservabilityWindow:         5,
		ConsecutiveFailureThreshold: 3,
	}

	manager, err := NewCircuitBreakerManager(
		StoreOption(mockStore),
		ClockOption(mockClock),
		ConfigOption(config),
		LoggerOption(log.NewLogger(os.Stdout)),
	)
	require.NoError(t, err)

	ctx := context.Background()
	pollResults := map[string]PollResult{
		"test1": {Key: "test1", Failures: 3, Successes: 7},
		"test2": {Key: "test2", Failures: 6, Successes: 4},
	}

	err = manager.sampleStore(ctx, pollResults)
	require.NoError(t, err)

	// Check if circuit breakers were created and updated correctly
	cb1, err := manager.GetCircuitBreakerWithError(ctx, "test1")
	require.NoError(t, err)
	require.Equal(t, StateClosed, cb1.State)
	require.Equal(t, uint64(10), cb1.Requests)
	require.Equal(t, uint64(3), cb1.TotalFailures)
	require.Equal(t, uint64(7), cb1.TotalSuccesses)

	cb2, err := manager.GetCircuitBreakerWithError(ctx, "test2")
	require.NoError(t, err)
	require.Equal(t, StateOpen, cb2.State)
	require.Equal(t, uint64(10), cb2.Requests)
	require.Equal(t, uint64(6), cb2.TotalFailures)
	require.Equal(t, uint64(4), cb2.TotalSuccesses)
}

func TestCircuitBreakerManager_UpdateCircuitBreakers(t *testing.T) {
	mockStore := NewTestStore()
	mockClock := clock.NewSimulatedClock(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	config := &CircuitBreakerConfig{
		SampleRate:                  1,
		BreakerTimeout:              30,
		FailureThreshold:            50,
		SuccessThreshold:            10,
		MinimumRequestCount:         10,
		ObservabilityWindow:         5,
		ConsecutiveFailureThreshold: 3,
	}

	manager, err := NewCircuitBreakerManager(
		StoreOption(mockStore),
		ClockOption(mockClock),
		ConfigOption(config),
		LoggerOption(log.NewLogger(os.Stdout)),
	)
	require.NoError(t, err)

	ctx := context.Background()
	breakers := map[string]CircuitBreaker{
		"breaker:test1": {
			Key:            "breaker:test1",
			State:          StateClosed,
			Requests:       10,
			TotalFailures:  3,
			TotalSuccesses: 7,
		},
		"breaker:test2": {
			Key:            "breaker:test2",
			State:          StateOpen,
			Requests:       10,
			TotalFailures:  6,
			TotalSuccesses: 4,
		},
	}

	err = manager.updateCircuitBreakers(ctx, breakers)
	require.NoError(t, err)

	// Check if circuit breakers were updated in the store
	cb1, err := manager.GetCircuitBreakerWithError(ctx, "test1")
	require.NoError(t, err)
	require.Equal(t, StateClosed, cb1.State)
	require.Equal(t, uint64(10), cb1.Requests)
	require.Equal(t, uint64(3), cb1.TotalFailures)
	require.Equal(t, uint64(7), cb1.TotalSuccesses)

	cb2, err := manager.GetCircuitBreakerWithError(ctx, "test2")
	require.NoError(t, err)
	require.Equal(t, StateOpen, cb2.State)
	require.Equal(t, uint64(10), cb2.Requests)
	require.Equal(t, uint64(6), cb2.TotalFailures)
	require.Equal(t, uint64(4), cb2.TotalSuccesses)
}

func TestCircuitBreakerManager_LoadCircuitBreakers_TestStore(t *testing.T) {
	mockStore := NewTestStore()
	mockClock := clock.NewSimulatedClock(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	config := &CircuitBreakerConfig{
		SampleRate:                  1,
		BreakerTimeout:              30,
		FailureThreshold:            50,
		SuccessThreshold:            10,
		MinimumRequestCount:         10,
		ObservabilityWindow:         5,
		ConsecutiveFailureThreshold: 3,
	}

	manager, err := NewCircuitBreakerManager(
		StoreOption(mockStore),
		ClockOption(mockClock),
		ConfigOption(config),
		LoggerOption(log.NewLogger(os.Stdout)),
	)
	require.NoError(t, err)

	ctx := context.Background()
	breakers := map[string]CircuitBreaker{
		"breaker:test1": {
			Key:            "test1",
			State:          StateClosed,
			Requests:       10,
			TotalFailures:  3,
			TotalSuccesses: 7,
		},
		"breaker:test2": {
			Key:            "test2",
			State:          StateOpen,
			Requests:       10,
			TotalFailures:  6,
			TotalSuccesses: 4,
		},
	}

	err = manager.updateCircuitBreakers(ctx, breakers)
	require.NoError(t, err)

	loadedBreakers, err := manager.loadCircuitBreakers(ctx)
	require.NoError(t, err)
	require.Len(t, loadedBreakers, 2)

	// Check if loaded circuit breakers match the original ones
	for _, cb := range loadedBreakers {
		originalCB, exists := breakers["breaker:"+cb.Key]
		require.True(t, exists)
		require.Equal(t, originalCB.State, cb.State)
		require.Equal(t, originalCB.Requests, cb.Requests)
		require.Equal(t, originalCB.TotalFailures, cb.TotalFailures)
		require.Equal(t, originalCB.TotalSuccesses, cb.TotalSuccesses)
	}
}

func TestCircuitBreakerManager_LoadCircuitBreakers_RedisStore(t *testing.T) {
	ctx := context.Background()

	re, err := getRedis(t)
	require.NoError(t, err)

	mockClock := clock.NewSimulatedClock(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	store := NewRedisStore(re, mockClock)

	keys, err := re.Keys(ctx, "breaker*").Result()
	require.NoError(t, err)

	for i := range keys {
		err = re.Del(ctx, keys[i]).Err()
		require.NoError(t, err)
	}

	config := &CircuitBreakerConfig{
		SampleRate:                  1,
		BreakerTimeout:              30,
		FailureThreshold:            50,
		SuccessThreshold:            10,
		MinimumRequestCount:         10,
		ObservabilityWindow:         5,
		ConsecutiveFailureThreshold: 3,
	}

	manager, err := NewCircuitBreakerManager(
		StoreOption(store),
		ClockOption(mockClock),
		ConfigOption(config),
		LoggerOption(log.NewLogger(os.Stdout)),
	)
	require.NoError(t, err)

	breakers := map[string]CircuitBreaker{
		"breaker:test1": {
			Key:            "test1",
			State:          StateClosed,
			Requests:       10,
			TotalFailures:  3,
			TotalSuccesses: 7,
		},
		"breaker:test2": {
			Key:            "test2",
			State:          StateOpen,
			Requests:       10,
			TotalFailures:  6,
			TotalSuccesses: 4,
		},
	}

	err = manager.updateCircuitBreakers(ctx, breakers)
	require.NoError(t, err)

	loadedBreakers, err := manager.loadCircuitBreakers(ctx)
	require.NoError(t, err)
	require.Len(t, loadedBreakers, 2)

	// Check if loaded circuit breakers match the original ones
	for _, cb := range loadedBreakers {
		originalCB, exists := breakers["breaker:"+cb.Key]
		require.True(t, exists)
		require.Equal(t, originalCB.State, cb.State)
		require.Equal(t, originalCB.Requests, cb.Requests)
		require.Equal(t, originalCB.TotalFailures, cb.TotalFailures)
		require.Equal(t, originalCB.TotalSuccesses, cb.TotalSuccesses)
	}
}

func TestCircuitBreakerManager_CanExecute(t *testing.T) {
	mockStore := NewTestStore()
	mockClock := clock.NewSimulatedClock(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	config := &CircuitBreakerConfig{
		SampleRate:                  1,
		BreakerTimeout:              30,
		FailureThreshold:            50,
		SuccessThreshold:            10,
		MinimumRequestCount:         10,
		ObservabilityWindow:         5,
		ConsecutiveFailureThreshold: 3,
	}

	manager, err := NewCircuitBreakerManager(
		StoreOption(mockStore),
		ClockOption(mockClock),
		ConfigOption(config),
		LoggerOption(log.NewLogger(os.Stdout)),
	)
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("Circuit Breaker Not Found", func(t *testing.T) {
		err := manager.CanExecute(ctx, "non_existent")
		require.NoError(t, err)
	})

	t.Run("Closed State", func(t *testing.T) {
		cb := CircuitBreaker{
			Key:   "test_closed",
			State: StateClosed,
		}
		err := manager.store.SetOne(ctx, "breaker:test_closed", cb, time.Minute)
		require.NoError(t, err)

		err = manager.CanExecute(ctx, "test_closed")
		require.NoError(t, err)
	})

	t.Run("Open State", func(t *testing.T) {
		cb := CircuitBreaker{
			Key:   "test_open",
			State: StateOpen,
		}
		err := manager.store.SetOne(ctx, "breaker:test_open", cb, time.Minute)
		require.NoError(t, err)

		err = manager.CanExecute(ctx, "test_open")
		require.Equal(t, ErrOpenState, err)
	})

	t.Run("Half-Open State with Too Many Failures", func(t *testing.T) {
		cb := CircuitBreaker{
			Key:         "test_half_open",
			State:       StateHalfOpen,
			FailureRate: 60,
			WillResetAt: time.Date(2020, 1, 1, 0, 1, 0, 0, time.UTC),
		}
		err := manager.store.SetOne(ctx, "breaker:test_half_open", cb, time.Minute)
		require.NoError(t, err)

		err = manager.CanExecute(ctx, "test_half_open")
		require.Equal(t, ErrTooManyRequests, err)
	})

	t.Run("Half-Open State with Acceptable Failures", func(t *testing.T) {
		cb := CircuitBreaker{
			Key:           "test_half_open_ok",
			State:         StateHalfOpen,
			TotalFailures: 4,
		}
		err := manager.store.SetOne(ctx, "breaker:test_half_open_ok", cb, time.Minute)
		require.NoError(t, err)

		err = manager.CanExecute(ctx, "test_half_open_ok")
		require.NoError(t, err)
	})
}

func TestCircuitBreakerManager_GetCircuitBreaker(t *testing.T) {
	mockStore := NewTestStore()
	mockClock := clock.NewSimulatedClock(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	config := &CircuitBreakerConfig{
		SampleRate:                  1,
		BreakerTimeout:              30,
		FailureThreshold:            50,
		SuccessThreshold:            10,
		MinimumRequestCount:         10,
		ObservabilityWindow:         5,
		ConsecutiveFailureThreshold: 3,
	}

	manager, err := NewCircuitBreakerManager(
		StoreOption(mockStore),
		ClockOption(mockClock),
		ConfigOption(config),
		LoggerOption(log.NewLogger(os.Stdout)),
	)
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("Circuit Breaker Not Found", func(t *testing.T) {
		_, err := manager.GetCircuitBreakerWithError(ctx, "non_existent")
		require.Equal(t, ErrCircuitBreakerNotFound, err)
	})

	t.Run("Circuit Breaker Found", func(t *testing.T) {
		cb := CircuitBreaker{
			Key:            "test_cb",
			State:          StateClosed,
			Requests:       10,
			TotalFailures:  3,
			TotalSuccesses: 7,
		}
		err := manager.store.SetOne(ctx, "breaker:test_cb", cb, time.Minute)
		require.NoError(t, err)

		retrievedCB, err := manager.GetCircuitBreakerWithError(ctx, "test_cb")
		require.NoError(t, err)
		require.Equal(t, cb.Key, retrievedCB.Key)
		require.Equal(t, cb.State, retrievedCB.State)
		require.Equal(t, cb.Requests, retrievedCB.Requests)
		require.Equal(t, cb.TotalFailures, retrievedCB.TotalFailures)
		require.Equal(t, cb.TotalSuccesses, retrievedCB.TotalSuccesses)
	})
}

func TestCircuitBreakerManager_SampleAndUpdate(t *testing.T) {
	mockStore := NewTestStore()
	mockClock := clock.NewSimulatedClock(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	config := &CircuitBreakerConfig{
		SampleRate:                  1,
		BreakerTimeout:              30,
		FailureThreshold:            50,
		SuccessThreshold:            10,
		MinimumRequestCount:         10,
		ObservabilityWindow:         5,
		ConsecutiveFailureThreshold: 3,
	}

	manager, err := NewCircuitBreakerManager(
		StoreOption(mockStore),
		ClockOption(mockClock),
		ConfigOption(config),
		LoggerOption(log.NewLogger(os.Stdout)),
	)
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("Sample and Update Success", func(t *testing.T) {
		pollFunc := func(ctx context.Context, lookBackDuration uint64, _ map[string]time.Time) (map[string]PollResult, error) {
			return map[string]PollResult{
				"test1": {Key: "test1", Failures: 3, Successes: 7},
				"test2": {Key: "test2", Failures: 6, Successes: 4},
			}, nil
		}

		err := manager.sampleAndUpdate(ctx, pollFunc)
		require.NoError(t, err)

		// Check if circuit breakers were created and updated correctly
		cb1, err := manager.GetCircuitBreakerWithError(ctx, "test1")
		require.NoError(t, err)
		require.Equal(t, StateClosed, cb1.State)
		require.Equal(t, uint64(10), cb1.Requests)
		require.Equal(t, uint64(3), cb1.TotalFailures)
		require.Equal(t, uint64(7), cb1.TotalSuccesses)

		cb2, err := manager.GetCircuitBreakerWithError(ctx, "test2")
		require.NoError(t, err)
		require.Equal(t, StateOpen, cb2.State)
		require.Equal(t, uint64(10), cb2.Requests)
		require.Equal(t, uint64(6), cb2.TotalFailures)
		require.Equal(t, uint64(4), cb2.TotalSuccesses)
	})

	t.Run("Sample and Update with Empty Results",
		func(t *testing.T) {
			pollFunc := func(ctx context.Context, lookBackDuration uint64, _ map[string]time.Time) (map[string]PollResult, error) {
				return map[string]PollResult{}, nil
			}

			err := manager.sampleAndUpdate(ctx, pollFunc)
			require.NoError(t, err)
		})

	t.Run("Sample and Update with Poll Function Error", func(t *testing.T) {
		pollFunc := func(ctx context.Context, lookBackDuration uint64, _ map[string]time.Time) (map[string]PollResult, error) {
			return nil, errors.New("poll function error")
		}

		err := manager.sampleAndUpdate(ctx, pollFunc)
		require.Error(t, err)
		require.Contains(t, err.Error(), "poll function failed")
	})
}

func TestCircuitBreakerManager_Start(t *testing.T) {
	mockStore := NewTestStore()
	mockClock := clock.NewSimulatedClock(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	config := &CircuitBreakerConfig{
		SampleRate:                  1,
		BreakerTimeout:              30,
		FailureThreshold:            50,
		SuccessThreshold:            10,
		MinimumRequestCount:         10,
		ObservabilityWindow:         5,
		ConsecutiveFailureThreshold: 3,
	}

	manager, err := NewCircuitBreakerManager(
		StoreOption(mockStore),
		ClockOption(mockClock),
		ConfigOption(config),
		LoggerOption(log.NewLogger(os.Stdout)),
	)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	pollCount := 0
	pollFunc := func(ctx context.Context, lookBackDuration uint64, _ map[string]time.Time) (map[string]PollResult, error) {
		pollCount++
		return map[string]PollResult{
			"test": {Key: "test", Failures: uint64(pollCount), Successes: 10 - uint64(pollCount)},
		}, nil
	}

	go manager.Start(ctx, pollFunc)

	// Wait for a few poll cycles
	time.Sleep(2500 * time.Millisecond)

	// Check if the circuit breaker was updated
	cb, err := manager.GetCircuitBreakerWithError(ctx, "test")
	require.NoError(t, err)
	require.NotNil(t, cb)
	require.Equal(t, uint64(10), cb.Requests)
	require.True(t, cb.TotalFailures > 0)
	require.True(t, cb.TotalSuccesses > 0)

	// Ensure the poll function was called multiple times
	require.True(t, pollCount > 1)
}
