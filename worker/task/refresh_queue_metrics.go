package task

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	log "github.com/frain-dev/convoy/pkg/logger"
)

func RefreshQueueMetricsSnapshot(lo log.Logger, db database.Database, locker JobLocker) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		cfg, err := config.Get()
		if err != nil || !cfg.Metrics.IsEnabled {
			return err
		}

		pg, ok := db.(*postgres.Postgres)
		if !ok {
			return fmt.Errorf("queue metrics snapshot requires postgres")
		}

		return skipIfLockBusy(locker.WithLock(ctx, "convoy:queue_metrics:mutex", 15*time.Minute, func(ctx context.Context) error {
			return pg.WriteQueueMetricsSnapshot(ctx)
		}))
	}
}
