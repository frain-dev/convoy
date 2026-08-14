package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	dbpostgres "github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/internal/pkg/limiter"
	"github.com/frain-dev/convoy/testenv"
)

var testInfra *testenv.Environment

func TestMain(m *testing.M) {
	res, cleanup, err := testenv.Launch(context.Background(), testenv.WithoutRedis())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to launch test infrastructure: %v\n", err)
		os.Exit(1)
	}
	testInfra = res
	code := m.Run()
	if err := cleanup(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to cleanup test infrastructure: %v\n", err)
	}
	os.Exit(code)
}

func setupLimiter(t *testing.T) *PostgresLimiter {
	t.Helper()
	conn, err := testInfra.CloneTestDatabase(t, "convoy")
	require.NoError(t, err)
	return New(dbpostgres.NewFromConnection(conn).GetDB())
}

func TestAllowThenDeny(t *testing.T) {
	l := setupLimiter(t)
	ctx := context.Background()
	key := "rl:" + ulid.Make().String()

	require.NoError(t, l.Allow(ctx, key, 1))
	err := l.Allow(ctx, key, 1)
	require.Error(t, err)
	require.Equal(t, limiter.ErrRateLimitExceeded, limiter.GetRawError(err))
	require.Greater(t, limiter.GetRetryAfter(err), time.Duration(0))
}

func TestZeroRateIsUnlimited(t *testing.T) {
	l := setupLimiter(t)
	ctx := context.Background()
	require.NoError(t, l.Allow(ctx, "rl:"+ulid.Make().String(), 0))
}

func TestConcurrentAllowDoesNotOverAdmit(t *testing.T) {
	l := setupLimiter(t)
	ctx := context.Background()
	key := "rl:" + ulid.Make().String()
	const (
		rate     = 20
		attempts = 100
	)

	start := make(chan struct{})
	results := make(chan error, attempts)
	var wg sync.WaitGroup
	for range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results <- l.AllowWithDuration(ctx, key, rate, 3600)
		}()
	}

	close(start)
	wg.Wait()
	close(results)

	allowed := 0
	for err := range results {
		if err == nil {
			allowed++
			continue
		}
		require.Equal(t, limiter.ErrRateLimitExceeded, limiter.GetRawError(err))
	}
	require.Equal(t, rate, allowed)
}

func TestAllowUsesLocalReservation(t *testing.T) {
	l := setupLimiter(t)
	ctx := context.Background()
	key := "rl:" + ulid.Make().String()

	require.NoError(t, l.AllowWithDuration(ctx, key, 100, 3600))

	var tokensAfterFirst float64
	err := l.db.GetContext(ctx, &tokensAfterFirst, `SELECT tokens FROM convoy.rate_limits WHERE key = $1`, key)
	require.NoError(t, err)

	require.NoError(t, l.AllowWithDuration(ctx, key, 100, 3600))

	var tokensAfterSecond float64
	err = l.db.GetContext(ctx, &tokensAfterSecond, `SELECT tokens FROM convoy.rate_limits WHERE key = $1`, key)
	require.NoError(t, err)
	require.Equal(t, tokensAfterFirst, tokensAfterSecond)
	require.InDelta(t, 68, tokensAfterSecond, 0.01)
}

func TestExpiredLocalReservationIsDiscarded(t *testing.T) {
	l := setupLimiter(t)
	ctx := context.Background()
	key := "rl:" + ulid.Make().String()
	now := time.Now()
	l.now = func() time.Time { return now }

	require.NoError(t, l.AllowWithDuration(ctx, key, 100, 3600))

	var tokensAfterFirst float64
	err := l.db.GetContext(ctx, &tokensAfterFirst, `SELECT tokens FROM convoy.rate_limits WHERE key = $1`, key)
	require.NoError(t, err)
	require.InDelta(t, 68, tokensAfterFirst, 0.01)

	now = now.Add(time.Hour + time.Second)
	require.NoError(t, l.AllowWithDuration(ctx, key, 100, 3600))

	var tokensAfterExpiry float64
	err = l.db.GetContext(ctx, &tokensAfterExpiry, `SELECT tokens FROM convoy.rate_limits WHERE key = $1`, key)
	require.NoError(t, err)
	require.Less(t, tokensAfterExpiry, tokensAfterFirst)
	require.InDelta(t, 36, tokensAfterExpiry, 0.1)
}

func TestConcurrentLimitersDoNotOverAdmitReservedTokens(t *testing.T) {
	first := setupLimiter(t)
	second := New(first.db)
	ctx := context.Background()
	key := "rl:" + ulid.Make().String()
	const (
		rate     = 20
		attempts = 100
	)

	start := make(chan struct{})
	results := make(chan error, attempts)
	var wg sync.WaitGroup
	for i := range attempts {
		wg.Add(1)
		go func(l *PostgresLimiter) {
			defer wg.Done()
			<-start
			results <- l.AllowWithDuration(ctx, key, rate, 3600)
		}([]*PostgresLimiter{first, second}[i%2])
	}

	close(start)
	wg.Wait()
	close(results)

	allowed := 0
	for err := range results {
		if err == nil {
			allowed++
			continue
		}
		require.Equal(t, limiter.ErrRateLimitExceeded, limiter.GetRawError(err))
	}
	require.Equal(t, rate, allowed)
}

func TestLimitChangeDiscardsOldReservation(t *testing.T) {
	l := setupLimiter(t)
	ctx := context.Background()
	key := "rl:" + ulid.Make().String()

	require.NoError(t, l.AllowWithDuration(ctx, key, 100, 3600))
	require.NoError(t, l.AllowWithDuration(ctx, key, 1, 3600))

	err := l.AllowWithDuration(ctx, key, 1, 3600)
	require.Error(t, err)
	require.Equal(t, limiter.ErrRateLimitExceeded, limiter.GetRawError(err))
}

func TestAllowRefillsTokens(t *testing.T) {
	l := setupLimiter(t)
	ctx := context.Background()
	key := "rl:" + ulid.Make().String()

	require.NoError(t, l.Allow(ctx, key, 1))
	require.Error(t, l.Allow(ctx, key, 1))

	_, err := l.db.ExecContext(ctx, `
		UPDATE convoy.rate_limits
		SET updated_at = NOW() - INTERVAL '2 seconds'
		WHERE key = $1`,
		key,
	)
	require.NoError(t, err)
	require.NoError(t, l.Allow(ctx, key, 1))
}

func TestAllowFailsClosedOnCancelledContext(t *testing.T) {
	l := setupLimiter(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := l.Allow(ctx, "rl:"+ulid.Make().String(), 1)
	require.Error(t, err)
	require.True(t, errors.Is(err, context.Canceled))
}
