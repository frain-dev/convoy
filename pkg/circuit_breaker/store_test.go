package circuit_breaker

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/frain-dev/convoy/pkg/clock"
	"github.com/stretchr/testify/require"
)

func TestRedisStore_Keys(t *testing.T) {
	ctx := context.Background()
	redisClient, err := getRedis(t)
	require.NoError(t, err)

	mockClock := clock.NewSimulatedClock(time.Now())
	store := NewRedisStore(redisClient, mockClock)

	// Clean up any existing keys
	existingKeys, err := redisClient.Keys(ctx, "test_keys*").Result()
	require.NoError(t, err)
	if len(existingKeys) > 0 {
		err = redisClient.Del(ctx, existingKeys...).Err()
		require.NoError(t, err)
	}

	// Set up test data
	testKeys := []string{"test_keys:1", "test_keys:2", "test_keys:3"}
	for _, key := range testKeys {
		err = redisClient.Set(ctx, key, "value", time.Minute).Err()
		require.NoError(t, err)
	}

	// Test Keys method
	keys, err := store.Keys(ctx, "test_keys")
	require.NoError(t, err)
	require.ElementsMatch(t, testKeys, keys)

	// Clean up
	err = redisClient.Del(ctx, testKeys...).Err()
	require.NoError(t, err)
}

func TestRedisStore_GetOne(t *testing.T) {
	ctx := context.Background()
	redisClient, err := getRedis(t)
	require.NoError(t, err)

	mockClock := clock.NewSimulatedClock(time.Now())
	store := NewRedisStore(redisClient, mockClock)

	t.Run("Existing Key", func(t *testing.T) {
		key := "test_get_one:existing"
		value := "test_value"
		err = redisClient.Set(ctx, key, value, time.Minute).Err()
		require.NoError(t, err)

		result, err := store.GetOne(ctx, key)
		require.NoError(t, err)
		require.Equal(t, value, result)

		err = redisClient.Del(ctx, key).Err()
		require.NoError(t, err)
	})

	t.Run("Non-existing Key", func(t *testing.T) {
		key := "test_get_one:non_existing"
		_, err := store.GetOne(ctx, key)
		require.Equal(t, ErrCircuitBreakerNotFound, err)
	})
}

func TestRedisStore_GetMany(t *testing.T) {
	ctx := context.Background()
	redisClient, err := getRedis(t)
	require.NoError(t, err)

	mockClock := clock.NewSimulatedClock(time.Now())
	store := NewRedisStore(redisClient, mockClock)

	// Set up test data
	testData := map[string]string{
		"test_get_many:1": "value1",
		"test_get_many:2": "value2",
		"test_get_many:3": "value3",
	}
	for key, value := range testData {
		err = redisClient.Set(ctx, key, value, time.Minute).Err()
		require.NoError(t, err)
	}

	keys := []string{"test_get_many:1", "test_get_many:2", "test_get_many:3", "test_get_many:non_existing"}
	results, err := store.GetMany(ctx, keys...)
	require.NoError(t, err)
	require.Len(t, results, 4)

	for i, key := range keys {
		if i < 3 {
			require.Equal(t, testData[key], results[i])
		} else {
			require.Nil(t, results[i])
		}
	}

	// Clean up
	err = redisClient.Del(ctx, "test_get_many:1", "test_get_many:2", "test_get_many:3").Err()
	require.NoError(t, err)
}

func TestRedisStore_SetOne(t *testing.T) {
	ctx := context.Background()
	redisClient, err := getRedis(t)
	require.NoError(t, err)

	mockClock := clock.NewSimulatedClock(time.Now())
	store := NewRedisStore(redisClient, mockClock)

	key := "test_set_one"
	value := "test_value"
	expiration := time.Minute

	err = store.SetOne(ctx, key, value, expiration)
	require.NoError(t, err)

	// Verify the value was set
	result, err := redisClient.Get(ctx, key).Result()
	require.NoError(t, err)
	require.Equal(t, value, result)

	// Verify the expiration was set
	ttl, err := redisClient.TTL(ctx, key).Result()
	require.NoError(t, err)
	require.True(t, ttl > 0 && ttl <= expiration)

	// Clean up
	err = redisClient.Del(ctx, key).Err()
	require.NoError(t, err)
}

func TestRedisStore_SetMany(t *testing.T) {
	ctx := context.Background()
	redisClient, err := getRedis(t)
	require.NoError(t, err)

	mockClock := clock.NewSimulatedClock(time.Now())
	store := NewRedisStore(redisClient, mockClock)

	breakers := map[string]CircuitBreaker{
		"test_set_many:1": {
			Key:   "test_set_many:1",
			State: StateClosed,
		},
		"test_set_many:2": {
			Key:   "test_set_many:2",
			State: StateOpen,
		},
	}
	expiration := time.Minute

	err = store.SetMany(ctx, breakers, expiration)
	require.NoError(t, err)

	// Verify the values were set
	for key, breaker := range breakers {
		result, err := redisClient.Get(ctx, key).Result()
		require.NoError(t, err)

		expectedValue, err := breaker.String()
		require.NoError(t, err)
		require.Equal(t, expectedValue, result)

		// Verify the expiration was set
		ttl, err := redisClient.TTL(ctx, key).Result()
		require.NoError(t, err)
		require.True(t, ttl > 0 && ttl <= expiration)
	}

	// Clean up
	keys := make([]string, 0, len(breakers))
	for key := range breakers {
		keys = append(keys, key)
	}
	err = redisClient.Del(ctx, keys...).Err()
	require.NoError(t, err)
}

func TestTestStore_Keys(t *testing.T) {
	store := NewTestStore()
	ctx := context.Background()

	// Add some test data
	store.store["test:1"] = CircuitBreaker{Key: "test:1"}
	store.store["test:2"] = CircuitBreaker{Key: "test:2"}
	store.store["other:1"] = CircuitBreaker{Key: "other:1"}

	keys, err := store.Keys(ctx, "test")
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"test:1", "test:2"}, keys)
}

func TestTestStore_GetOne(t *testing.T) {
	store := NewTestStore()
	ctx := context.Background()

	t.Run("Existing Key", func(t *testing.T) {
		cb := CircuitBreaker{Key: "test", State: StateClosed}
		store.store["test"] = cb

		result, err := store.GetOne(ctx, "test")
		require.NoError(t, err)

		expectedValue, _ := cb.String()
		require.Equal(t, expectedValue, result)
	})

	t.Run("Non-existing Key", func(t *testing.T) {
		_, err := store.GetOne(ctx, "non_existing")
		require.Equal(t, ErrCircuitBreakerNotFound, err)
	})
}

func TestTestStore_GetMany(t *testing.T) {
	store := NewTestStore()
	ctx := context.Background()

	cb1 := CircuitBreaker{Key: "test1", State: StateClosed}
	cb2 := CircuitBreaker{Key: "test2", State: StateOpen}
	store.store["test1"] = cb1
	store.store["test2"] = cb2

	results, err := store.GetMany(ctx, "test1", "test2", "non_existing")
	require.NoError(t, err)
	require.Len(t, results, 3)

	require.Equal(t, cb1, results[0])
	require.Equal(t, cb2, results[1])
	require.Nil(t, results[2])
}

func TestTestStore_SetOne(t *testing.T) {
	store := NewTestStore()
	ctx := context.Background()

	cb := CircuitBreaker{Key: "test", State: StateClosed}
	err := store.SetOne(ctx, "test", cb, time.Minute)
	require.NoError(t, err)

	storedCB, ok := store.store["test"]
	require.True(t, ok)
	require.Equal(t, cb, storedCB)
}

func TestTestStore_SetMany(t *testing.T) {
	store := NewTestStore()
	ctx := context.Background()

	breakers := map[string]CircuitBreaker{
		"test1": {Key: "test1", State: StateClosed},
		"test2": {Key: "test2", State: StateOpen},
	}

	err := store.SetMany(ctx, breakers, time.Minute)
	require.NoError(t, err)

	for key, cb := range breakers {
		storedCB, ok := store.store[key]
		require.True(t, ok)
		require.Equal(t, cb, storedCB)
	}
}

func TestTestStore_Concurrency(t *testing.T) {
	store := NewTestStore()
	ctx := context.Background()
	wg := &sync.WaitGroup{}

	// Test concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		key := fmt.Sprintf("key_%d", i)
		err := store.SetOne(ctx, key, CircuitBreaker{Key: key, State: StateClosed}, time.Minute)
		require.NoError(t, err)
	}

	go func() {
		for i := 0; i < 100; i++ {
			_, err := store.GetOne(ctx, fmt.Sprintf("key_%d", i))
			require.NoError(t, err)
			wg.Done()
		}
	}()

	// If there's a race condition, this test might panic or deadlock
	time.Sleep(100 * time.Millisecond)
	wg.Wait()
}
