package broker

import (
	"testing"

	"github.com/frain-dev/convoy/api/types"
	"github.com/stretchr/testify/require"
)

func TestDependenciesApplyToAPIOptions(t *testing.T) {
	deps := &Dependencies{}
	o := &types.APIOptions{}
	deps.ApplyToAPIOptions(o)

	require.Equal(t, deps.Queue, o.Queue)
	require.Equal(t, deps.QueueMonitor, o.QueueMonitor)
	require.Equal(t, deps.QueueInspector, o.QueueInspector)
	require.Equal(t, deps.Cache, o.Cache)
	require.Equal(t, deps.Cache, o.QueueSessionStore)
	require.Equal(t, deps.RateLimiter, o.Rate)
	require.Equal(t, deps.CircuitBreakerStore, o.CircuitBreakerStore)
	require.Equal(t, deps.TrialEvents, o.TrialEvents)
	require.Equal(t, deps.Acker, o.Acker)
	require.Equal(t, deps.ResendClaims, o.ResendClaims)
	require.Equal(t, deps.JobLocker, o.UsageLocker)
	require.Equal(t, deps.BatchTracker, o.BatchTracker)
}

func TestDependenciesApplyToAPIOptions_nilSafe(t *testing.T) {
	var d *Dependencies
	o := &types.APIOptions{}
	require.NotPanics(t, func() { d.ApplyToAPIOptions(o) })
	require.NotPanics(t, func() { (&Dependencies{}).ApplyToAPIOptions(nil) })
}
