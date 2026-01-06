package rlimiter

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/testenv"
)

var testInfra *testenv.Environment

func TestMain(m *testing.M) {
	res, cleanup, err := testenv.Launch(context.Background())
	if err != nil {
		log.Fatalf("Failed to launch test infrastructure: %v", err)
	}

	testInfra = res

	code := m.Run()

	if err = cleanup(); err != nil {
		log.Fatalf("Failed to cleanup test infrastructure: %v", err)
	}

	os.Exit(code)
}

type RedisLimiterIntegrationTestSuite struct {
	suite.Suite
	limiter *RedisLimiter
}

func (s *RedisLimiterIntegrationTestSuite) SetupTest() {
	// Each test gets a fresh Redis client
	rd, err := testInfra.NewRedisClient(s.T(), 0)
	s.Require().NoError(err)

	// Flush the database to ensure a clean state
	err = rd.FlushDB(context.Background()).Err()
	s.Require().NoError(err)

	s.limiter = NewLimiterFromRedisClient(rd)
}

func (s *RedisLimiterIntegrationTestSuite) Test_RateLimitAllow() {
	vals := []int{10, 20}
	limit := 2

	for _, duration := range vals {
		uid := ulid.Make().String()
		s.Run(fmt.Sprintf("%s-%v", uid, duration), func() {
			err := s.limiter.AllowWithDuration(context.Background(), uid, limit, duration)
			require.NoError(s.T(), err)

			dur := GetRetryAfter(err)
			require.Equal(s.T(), time.Duration(0), dur)

			err = s.limiter.AllowWithDuration(context.Background(), uid, limit, duration)
			require.NoError(s.T(), err)

			dur = GetRetryAfter(err)
			require.Equal(s.T(), time.Duration(0), dur)

			err = s.limiter.AllowWithDuration(context.Background(), uid, limit, duration)
			require.Error(s.T(), err)
			require.ErrorIs(s.T(), GetRawError(err), ErrRateLimitExceeded)

			dur = GetRetryAfter(err)
			require.LessOrEqual(s.T(), time.Duration(duration), dur)

			err = s.limiter.AllowWithDuration(context.Background(), uid, limit, duration)
			require.Error(s.T(), err)
			require.ErrorIs(s.T(), GetRawError(err), ErrRateLimitExceeded)

			dur = GetRetryAfter(err)
			require.LessOrEqual(s.T(), time.Duration(duration), dur)
		})
	}
}

func TestRedisLimiterIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(RedisLimiterIntegrationTestSuite))
}
