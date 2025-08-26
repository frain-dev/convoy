package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"testing"
	"time"

	"github.com/frain-dev/convoy/internal/pkg/cli"
	cb "github.com/frain-dev/convoy/pkg/circuit_breaker"
	"github.com/frain-dev/convoy/pkg/clock"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func getRedisForTest(t *testing.T) redis.UniversalClient {
	t.Helper()
	opts, err := redis.ParseURL("redis://localhost:6379")
	if err != nil {
		t.Skipf("skipping: cannot parse redis URL: %v", err)
	}
	client := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("skipping: redis not available: %v", err)
	}
	return client
}

func captureStdout(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	fn()
	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()
	return buf.String()
}

func TestCircuitBreakersGet_PrintsJSON(t *testing.T) {
	redisClient := getRedisForTest(t)

	// clear any existing breaker key
	ctx := context.Background()
	_ = redisClient.Del(ctx, "breaker:test-breaker").Err()

	// seed a breaker directly into the store
	store := cb.NewRedisStore(redisClient, clock.NewRealClock())
	breaker := cb.CircuitBreaker{
		Key:                 "test-breaker",
		TenantId:            "proj-123",
		State:               cb.StateClosed,
		Requests:            10,
		TotalFailures:       3,
		TotalSuccesses:      7,
		ConsecutiveFailures: 1,
		NotificationsSent:   0,
		FailureRate:         30,
		SuccessRate:         70,
		WillResetAt:         time.Now().Add(1 * time.Hour),
	}
	// Store expects a string; use SetMany which serializes via String()
	err := store.SetMany(ctx, map[string]cb.CircuitBreaker{"breaker:test-breaker": breaker}, time.Minute)
	require.NoError(t, err)

	app := &cli.App{Redis: redisClient}
	cmd := AddCircuitBreakersGetCommand(app)

	out := captureStdout(func() {
		runErr := cmd.RunE(cmd, []string{"test-breaker"})
		require.NoError(t, runErr)
	})

	t.Logf("get output (no prefix):\n%s", out)

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &got))
	require.Equal(t, "breaker:test-breaker", got["key"])
	require.Equal(t, "proj-123", got["tenant_id"])
	// spot-check a few numeric fields exist
	_, hasReq := got["requests"]
	_, hasFails := got["total_failures"]
	require.True(t, hasReq)
	require.True(t, hasFails)
}

func TestCircuitBreakersGet_TrimsPrefix(t *testing.T) {
	redisClient := getRedisForTest(t)
	ctx := context.Background()
	_ = redisClient.Del(ctx, "breaker:test-breaker2").Err()

	store := cb.NewRedisStore(redisClient, clock.NewRealClock())
	breaker := cb.CircuitBreaker{Key: "test-breaker2", TenantId: "proj-xyz"}
	err := store.SetMany(ctx, map[string]cb.CircuitBreaker{"breaker:test-breaker2": breaker}, time.Minute)
	require.NoError(t, err)

	app := &cli.App{Redis: redisClient}
	cmd := AddCircuitBreakersGetCommand(app)

	out := captureStdout(func() {
		runErr := cmd.RunE(cmd, []string{"breaker:test-breaker2"})
		require.NoError(t, runErr)
	})

	t.Logf("get output (with prefix):\n%s", out)

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &got))
	require.Equal(t, "breaker:test-breaker2", got["key"])
	require.Equal(t, "proj-xyz", got["tenant_id"])
}

func TestCircuitBreakersGet_NotFound(t *testing.T) {
	redisClient := getRedisForTest(t)
	app := &cli.App{Redis: redisClient}
	cmd := AddCircuitBreakersGetCommand(app)
	err := cmd.RunE(cmd, []string{"does-not-exist"})
	require.Error(t, err)
	t.Logf("get error (not found): %v", err)
}
