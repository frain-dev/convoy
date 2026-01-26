package task

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v9"
	"github.com/olamilekan000/surge/surge/job"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
	"github.com/frain-dev/convoy/pkg/log"
)

func RefreshMetricsMaterializedViews(db database.Database, rd *rdb.Redis) func(context.Context, *job.JobEnvelope) error {
	pool := goredis.NewPool(rd.Client())
	rs := redsync.New(pool)

	return func(ctx context.Context, jobEnvelope *job.JobEnvelope) error {
		const mutexName = "convoy:refresh_metrics_mv:mutex"
		mutex := rs.NewMutex(mutexName, redsync.WithExpiry(25*time.Minute), redsync.WithTries(1))

		lockCtx, cancel := context.WithTimeout(ctx, time.Second*2)
		defer cancel()

		err := mutex.LockContext(lockCtx)
		if err != nil {
			return fmt.Errorf("failed to obtain lock: %v", err)
		}

		defer func() {
			unlockCtx, unlockCancel := context.WithTimeout(ctx, time.Second*2)
			defer unlockCancel()

			ok, err := mutex.UnlockContext(unlockCtx)
			if !ok || err != nil {
				log.FromContext(ctx).WithError(err).Error("failed to release lock")
			}
		}()

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
				log.FromContext(ctx).WithError(err).Errorf("failed to refresh materialized view: %s", q.name)
				// Continue with other views even if one fails
				continue
			}
			log.FromContext(ctx).Infof("refreshed materialized view: %s", q.name)
		}

		log.FromContext(ctx).Infof("refreshed all metrics materialized views in %v", time.Since(start))
		return nil
	}
}
