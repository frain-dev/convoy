package task

import (
	"context"
	"errors"
	"time"

	"github.com/hibiken/asynq"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/internal/event_deliveries"
	log "github.com/frain-dev/convoy/pkg/logger"
)

func RefreshEventDeliveryDailyCounts(lo log.Logger, db database.Database, locker JobLocker) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		return skipIfLockBusy(locker.WithLock(ctx, "convoy:daily_counts:mutex", 15*time.Minute, func(ctx context.Context) error {
			svc := event_deliveries.New(lo, db)
			if err := svc.RefreshRecentDailyCounts(ctx); err != nil {
				return err
			}
			done, err := svc.AdvanceDailyCountsBackfill(ctx)
			if err != nil {
				return err
			}
			if done {
				return svc.PruneDailyCountsBeforeLiveHistory(ctx)
			}
			return nil
		}))
	}
}

func skipIfLockBusy(err error) error {
	if errors.Is(err, ErrLockBusy) {
		return nil
	}
	return err
}
