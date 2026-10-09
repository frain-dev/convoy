package dynamiceventack

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	mcache "github.com/frain-dev/convoy/cache/memory"
)

func redisOrSkip(t *testing.T) *redis.Client {
	t.Helper()
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379", DialTimeout: 300 * time.Millisecond})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("redis unavailable: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestPublishWaitSuccess(t *testing.T) {
	rdb := redisOrSkip(t)
	ctx := context.Background()
	projectID, eventID := "proj-ack", "evt-ok"

	require.NoError(t, Publish(ctx, rdb, projectID, eventID, Result{OK: true}))

	got, err := Wait(ctx, rdb, projectID, eventID, 2*time.Second)
	require.NoError(t, err)
	require.True(t, got.OK)
}

func TestPublishWaitError(t *testing.T) {
	rdb := redisOrSkip(t)
	ctx := context.Background()
	projectID, eventID := "proj-ack", "evt-err"

	require.NoError(t, Publish(ctx, rdb, projectID, eventID, Result{OK: false, Error: "bad template"}))

	got, err := Wait(ctx, rdb, projectID, eventID, 2*time.Second)
	require.NoError(t, err)
	require.False(t, got.OK)
	require.Equal(t, "bad template", got.Error)
}

func TestWaitTimeout(t *testing.T) {
	rdb := redisOrSkip(t)
	ctx := context.Background()

	_, err := Wait(ctx, rdb, "proj-ack", "evt-missing", 200*time.Millisecond)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrTimeout))
}

func TestWaitNilRedis(t *testing.T) {
	_, err := Wait(context.Background(), nil, "p", "e", time.Second)
	require.ErrorIs(t, err, ErrNilRedis)
}

func TestPublishNilRedis(t *testing.T) {
	err := Publish(context.Background(), nil, "p", "e", Result{OK: true})
	require.ErrorIs(t, err, ErrNilRedis)
}

func TestCachePublishWaitSuccess(t *testing.T) {
	acker := NewCacheAcker(mcache.NewMemoryCache())
	ctx := context.Background()

	require.NoError(t, acker.Publish(ctx, "project", "event", Result{OK: true}))

	got, err := acker.Wait(ctx, "project", "event", time.Second)
	require.NoError(t, err)
	require.True(t, got.OK)
}

func TestCachePublishWaitError(t *testing.T) {
	acker := NewCacheAcker(mcache.NewMemoryCache())
	ctx := context.Background()

	require.NoError(t, acker.Publish(ctx, "project", "event", Result{Error: "bad template"}))

	got, err := acker.Wait(ctx, "project", "event", time.Second)
	require.NoError(t, err)
	require.False(t, got.OK)
	require.Equal(t, "bad template", got.Error)
}

func TestCacheAckIsConsumedOnce(t *testing.T) {
	acker := NewCacheAcker(mcache.NewMemoryCache())
	ctx := context.Background()
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup

	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := acker.Wait(ctx, "project", "one-shot", 150*time.Millisecond)
			results <- err
		}()
	}

	close(start)
	time.Sleep(10 * time.Millisecond)
	require.NoError(t, acker.Publish(ctx, "project", "one-shot", Result{OK: true}))
	wg.Wait()
	close(results)

	succeeded, timedOut := 0, 0
	for err := range results {
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, ErrTimeout):
			timedOut++
		default:
			require.NoError(t, err)
		}
	}
	require.Equal(t, 1, succeeded)
	require.Equal(t, 1, timedOut)
}

func TestCacheWaitTimeout(t *testing.T) {
	acker := NewCacheAcker(mcache.NewMemoryCache())

	_, err := acker.Wait(context.Background(), "project", "missing", 75*time.Millisecond)
	require.ErrorIs(t, err, ErrTimeout)
}

func TestCacheAckerRequiresCache(t *testing.T) {
	acker := NewCacheAcker(nil)
	require.ErrorIs(t, acker.Publish(context.Background(), "project", "event", Result{OK: true}), ErrNilCache)

	_, err := acker.Wait(context.Background(), "project", "event", time.Second)
	require.ErrorIs(t, err, ErrNilCache)
}
