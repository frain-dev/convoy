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

	c := CircuitBreakerConfig{
		SampleTime:                  2,
		ErrorTimeout:                30,
		FailureThreshold:            10,
		FailureCount:                1,
		SuccessThreshold:            1,
		ObservabilityWindow:         5,
		NotificationThresholds:      []int{10},
		ConsecutiveFailureThreshold: 10,
	}
	b := NewCircuitBreakerManager(re.Client(), db.GetDB(), testClock, c)

	endpointId := "endpoint-1"
	pollResults := [][]PollResult{
		{
			PollResult{
				EndpointID: endpointId,
				Failures:   1,
				Successes:  0,
			},
		},
		{
			PollResult{
				EndpointID: endpointId,
				Failures:   1,
				Successes:  0,
			},
		},
		{
			PollResult{
				EndpointID: endpointId,
				Failures:   0,
				Successes:  1,
			},
		},
		{
			PollResult{
				EndpointID: endpointId,
				Failures:   0,
				Successes:  1,
			},
		},
	}

	for i := 0; i < len(pollResults); i++ {
		innerErr := b.sampleEventsAndUpdateState(ctx, pollResults[i])
		require.NoError(t, innerErr)

		testClock.AdvanceTime(time.Minute)
	}

	breakers, innerErr := b.loadCircuitBreakerStateFromRedis(ctx)
	require.NoError(t, innerErr)

	for i := 0; i < len(breakers); i++ {
		require.Equal(t, breakers[i].State, StateClosed)
	}
}
