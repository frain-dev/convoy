package dynamiceventack

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
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
