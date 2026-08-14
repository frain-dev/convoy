package task

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"github.com/frain-dev/convoy/database"
	log "github.com/frain-dev/convoy/pkg/logger"
)

func RefreshMetricsMaterializedViews(db database.Database, locker JobLocker, logger log.Logger) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		return locker.WithLock(ctx, "convoy:refresh_metrics_mv:mutex", 25*time.Minute, func(ctx context.Context) error {
			start := time.Now()
			queries := []struct {
				name string
				sql  string
			}{
				{
					name: "event_queue_metrics_mv",
					sql:  "REFRESH MATERIALIZED VIEW CONCURRENTLY convoy.event_queue_metrics_mv",
				},
				{
					name: "event_delivery_queue_metrics_mv",
					sql:  "REFRESH MATERIALIZED VIEW CONCURRENTLY convoy.event_delivery_queue_metrics_mv",
				},
				{
					name: "event_queue_backlog_metrics_mv",
					sql:  "REFRESH MATERIALIZED VIEW CONCURRENTLY convoy.event_queue_backlog_metrics_mv",
				},
				{
					name: "event_endpoint_backlog_metrics_mv",
					sql:  "REFRESH MATERIALIZED VIEW CONCURRENTLY convoy.event_endpoint_backlog_metrics_mv",
				},
			}

			for _, q := range queries {
				refreshCtx, refreshCancel := context.WithTimeout(ctx, 5*time.Minute)
				_, err := db.GetDB().ExecContext(refreshCtx, q.sql)
				refreshCancel()
				if err != nil {
					logger.ErrorContext(ctx, fmt.Sprintf("failed to refresh materialized view: %s: %v", q.name, err))
					continue
				}
				logger.InfoContext(ctx, fmt.Sprintf("refreshed materialized view: %s", q.name))
			}

			logger.InfoContext(ctx, fmt.Sprintf("refreshed all metrics materialized views in %v", time.Since(start)))
			return nil
		})
	}
}
