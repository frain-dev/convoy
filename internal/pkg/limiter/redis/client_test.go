//go:build integration
// +build integration

package rlimiter

import (
	"context"
	"fmt"
	"github.com/frain-dev/convoy/config"
	"github.com/oklog/ulid/v2"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func getDSN() []string {
	port, _ := strconv.Atoi(os.Getenv("TEST_REDIS_PORT"))
	c := config.RedisConfiguration{
		Scheme: "redis",
		Host:   os.Getenv("TEST_REDIS_HOST"),
		Port:   port,
	}
	return c.BuildDsn()
}

func Test_RateLimitAllow(t *testing.T) {
	dsn := getDSN()

	vals := []int{10, 20}
	limit := 2

	for _, duration := range vals {
		uid := ulid.Make().String()
		t.Run(fmt.Sprintf("%s-%v", uid, duration), func(t *testing.T) {
			limiter, err := NewRedisLimiter(dsn)
			require.NoError(t, err)

			err = limiter.AllowWithDuration(context.Background(), uid, limit, duration)
			require.NoError(t, err)

			dur := GetRetryAfter(err)
			require.Equal(t, time.Duration(0), dur)

			err = limiter.AllowWithDuration(context.Background(), uid, limit, duration)
			require.NoError(t, err)

			dur = GetRetryAfter(err)
			require.Equal(t, time.Duration(0), dur)

			err = limiter.AllowWithDuration(context.Background(), uid, limit, duration)
			require.Error(t, err)
			require.ErrorIs(t, GetRawError(err), ErrRateLimitExceeded)

			dur = GetRetryAfter(err)
			require.LessOrEqual(t, time.Duration(duration), dur)

			err = limiter.AllowWithDuration(context.Background(), uid, limit, duration)
			require.Error(t, err)
			require.ErrorIs(t, GetRawError(err), ErrRateLimitExceeded)

			dur = GetRetryAfter(err)
			require.LessOrEqual(t, time.Duration(duration), dur)

		})
	}
}
