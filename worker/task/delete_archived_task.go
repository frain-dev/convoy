package task

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/queue"
)

func DeleteArchivedTasks(r queue.Queuer, locker JobLocker, logger log.Logger) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		// One bulk DELETE of archived/completed queue rows (Postgres) or seven
		// inspector sweeps (Redis); 10m covers a large nightly backlog.
		return locker.WithLock(ctx, "convoy:delete_archived_tasks:mutex", 10*time.Minute, func(ctx context.Context) error {
			archiver, ok := r.(queue.Archiver)
			if !ok {
				return fmt.Errorf("queue does not support deleting archived tasks")
			}
			if err := archiver.DeleteArchived(ctx); err != nil {
				logger.ErrorContext(ctx, fmt.Sprintf("failed to delete archived queue jobs: %v", err))
			}
			return nil
		})
	}
}
