package task

import (
	"context"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/config"
	log "github.com/frain-dev/convoy/pkg/logger"
)

type panicLocker struct{}

func (panicLocker) WithLock(context.Context, string, time.Duration, func(context.Context) error) error {
	panic("lock must not run when metrics are disabled")
}

func TestRefreshQueueMetricsSnapshotDisabled(t *testing.T) {
	t.Setenv("CONVOY_JWT_SECRET", "test-access-secret")
	t.Setenv("CONVOY_JWT_REFRESH_SECRET", "test-refresh-secret")
	require.NoError(t, config.LoadConfig(""))
	require.NoError(t, config.Override(&config.Configuration{
		Metrics: config.MetricsConfiguration{IsEnabled: false},
	}))

	handler := RefreshQueueMetricsSnapshot(log.New("test", log.LevelError), nil, panicLocker{})
	require.NoError(t, handler(context.Background(), &asynq.Task{}))
}
