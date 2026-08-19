package task

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"github.com/frain-dev/convoy/internal/pkg/retention"
	log "github.com/frain-dev/convoy/pkg/logger"
)

func RetentionPolicies(locker JobLocker, ret retention.Retentioner, logger log.Logger) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		return locker.WithLock(ctx, "convoy:retention:mutex", 30*time.Minute, func(ctx context.Context) error {
			c := time.Now()
			if err := ret.Perform(ctx); err != nil {
				return err
			}
			logger.InfoContext(ctx, fmt.Sprintf("Retention job took %f minutes to run", time.Since(c).Minutes()))
			return nil
		})
	}
}
