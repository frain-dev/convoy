package circuit_breaker

import (
	"context"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
	"github.com/frain-dev/convoy/pkg/clock"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestNewCircuitBreaker(t *testing.T) {
	ctx := context.Background()

	re, err := rdb.NewClient([]string{"redis://localhost:6379"})
	require.NoError(t, err)

	keys, err := re.Client().Keys(ctx, "breaker*").Result()
	require.NoError(t, err)

	err = re.Client().Del(ctx, keys...).Err()
	require.NoError(t, err)

	db, err := postgres.NewDB(config.Configuration{
		Database: config.DatabaseConfiguration{
			Type:     config.PostgresDatabaseProvider,
			Scheme:   "postgres",
			Host:     "localhost",
			Username: "postgres",
			Password: "postgres",
			Database: "endpoint_fix",
			Options:  "sslmode=disable&connect_timeout=30",
			Port:     5432,
		},
	})
	require.NoError(t, err)

	testClock := clock.NewSimulatedClock(time.Now())

	b := NewCircuitBreakerManager(re.Client(), db.GetDB(), testClock)

	endpointId := "endpoint-1"
	pollResults := [][]DBPollResult{
		{
			DBPollResult{
				EndpointID: endpointId,
				Failures:   1,
				Successes:  0,
			},
		},
		{
			DBPollResult{
				EndpointID: endpointId,
				Failures:   1,
				Successes:  0,
			},
		},
		{
			DBPollResult{
				EndpointID: endpointId,
				Failures:   0,
				Successes:  0,
			},
		},
		{
			DBPollResult{
				EndpointID: endpointId,
				Failures:   0,
				Successes:  1,
			},
		},
	}

	for i := 0; i < len(pollResults); i++ {
		innerErr := b.sampleEventsAndUpdateState(ctx, pollResults[i])
		require.NoError(t, innerErr)

		breakers, innerErr := b.loadCircuitBreakerStateFromRedis(ctx)
		require.NoError(t, innerErr)

		require.Equal(t, len(breakers), 1)
		t.Log(breakers)

		testClock.AdvanceTime(time.Minute)
	}
}
