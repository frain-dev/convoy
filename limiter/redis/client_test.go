//go:build integration
// +build integration

package rlimiter

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

func getDSN() string {
	return os.Getenv("TEST_REDIS_DSN")
}

func flushRedis(dsn string) error {
	opts, err := redis.ParseURL(dsn)
	if err != nil {
		return err
	}

	client := redis.NewClient(opts)

	_, err = client.FlushAll(context.Background()).Result()

	return err
}

func Test_RateLimitAllow(t *testing.T) {
	dsn := getDSN()

	vals := []time.Duration{time.Minute, time.Hour}

	for _, duration := range vals {
		t.Run(fmt.Sprintf(" %v", duration), func(t *testing.T) {

			err := flushRedis(dsn)
			require.NoError(t, err)

			limiter, err := NewRedisLimiter(dsn)
			require.NoError(t, err)

			res, err := limiter.Allow(context.Background(), "UID", 2, int(duration))
			require.NoError(t, err)

			require.Equal(t, 2, res.Limit.Rate)
			require.Equal(t, 1, res.Remaining)
			require.Equal(t, res.RetryAfter, time.Duration(-1))

			res, err = limiter.Allow(context.Background(), "UID", 2, int(duration))
			require.NoError(t, err)

			require.Equal(t, 2, res.Limit.Rate)
			require.Equal(t, 0, res.Remaining)
			require.Equal(t, res.RetryAfter, time.Duration(-1))

			res, err = limiter.Allow(context.Background(), "UID", 2, int(duration))
			require.NoError(t, err)

			require.Equal(t, 2, res.Limit.Rate)
			require.Equal(t, 0, res.Remaining)
			require.LessOrEqual(t, int(res.ResetAfter), int(duration))
			require.Greater(t, int(res.ResetAfter), int(time.Duration(0)))

			res, err = limiter.Allow(context.Background(), "UID", 2, int(duration))
			require.NoError(t, err)

			require.Equal(t, 2, res.Limit.Rate)
			require.Equal(t, 0, res.Remaining)
			require.LessOrEqual(t, int(res.RetryAfter), int(duration/2))
			require.Greater(t, int(res.RetryAfter), int(time.Duration(0)))
		})
	}
}
