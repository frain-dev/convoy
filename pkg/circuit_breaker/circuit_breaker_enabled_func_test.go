package circuit_breaker

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/pkg/clock"
	log "github.com/frain-dev/convoy/pkg/logger"
)

func newEnabledFuncTestManager(t *testing.T, enabled func(context.Context) bool) *CircuitBreakerManager {
	t.Helper()

	c := &CircuitBreakerConfig{
		SampleRate:                  1,
		BreakerTimeout:              30,
		FailureThreshold:            70,
		SuccessThreshold:            5,
		ObservabilityWindow:         5,
		MinimumRequestCount:         10,
		ConsecutiveFailureThreshold: 10,
	}

	manager, err := NewCircuitBreakerManager(
		StoreOption(NewTestStore()),
		ClockOption(clock.NewSimulatedClock(time.Now())),
		ConfigProviderOption(func(string) *CircuitBreakerConfig { return c }),
		LoggerOption(log.New("convoy", log.LevelInfo)),
		MasterConfigOption(*c),
		SkipSleepOption(true),
		EnabledFuncOption(enabled),
	)
	require.NoError(t, err)
	return manager
}

func TestCircuitBreakerManager_EnabledFuncSkipsTick(t *testing.T) {
	manager := newEnabledFuncTestManager(t, func(context.Context) bool { return false })

	polled := false
	pollFunc := func(context.Context, uint64, map[string]time.Time) (map[string]PollResult, error) {
		polled = true
		return nil, nil
	}

	err := manager.sampleAndUpdate(context.Background(), pollFunc)
	require.NoError(t, err)
	require.False(t, polled, "pollFunc must not run when EnabledFunc reports disabled")
}

func TestCircuitBreakerManager_EnabledFuncRunsTick(t *testing.T) {
	manager := newEnabledFuncTestManager(t, func(context.Context) bool { return true })

	polled := false
	pollFunc := func(context.Context, uint64, map[string]time.Time) (map[string]PollResult, error) {
		polled = true
		return nil, nil // no results, exits before sampleStore
	}

	err := manager.sampleAndUpdate(context.Background(), pollFunc)
	require.NoError(t, err)
	require.True(t, polled, "pollFunc must run when EnabledFunc reports enabled")
}
