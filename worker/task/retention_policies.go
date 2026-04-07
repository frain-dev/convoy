package task

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v9"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"

	"github.com/frain-dev/convoy/internal/pkg/retention"
	log "github.com/frain-dev/convoy/pkg/logger"
)

func RetentionPolicies(rd redis.UniversalClient, ret retention.Retentioner, logger log.Logger) func(context.Context, *asynq.Task) error {
	pool := goredis.NewPool(rd)
	rs := redsync.New(pool)

	return func(ctx context.Context, t *asynq.Task) error {
		const mutexName = "convoy:retention:mutex"
		mutex := rs.NewMutex(mutexName, redsync.WithExpiry(30*time.Minute), redsync.WithTries(1))

		lockCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		err := mutex.LockContext(lockCtx)
		if err != nil {
			return fmt.Errorf("failed to obtain lock: %v", err)
		}

		// Renew lock periodically to prevent expiry during long-running retention
		renewDone := make(chan struct{})
		go func() {
			ticker := time.NewTicker(10 * time.Minute)
			defer ticker.Stop()
			for {
				select {
				case <-renewDone:
					return
				case <-ticker.C:
					renewCtx, renewCancel := context.WithTimeout(ctx, 10*time.Second)
					_, _ = mutex.ExtendContext(renewCtx)
					renewCancel()
				}
			}
		}()

		defer func() {
			close(renewDone)
			_lockCtx, _cancel := context.WithTimeout(ctx, time.Second*2)
			defer _cancel()

			ok, _err := mutex.UnlockContext(_lockCtx)
			if !ok || _err != nil {
				logger.ErrorContext(ctx, "failed to release lock", "error", _err)
			}
		}()

		c := time.Now()
		err = ret.Perform(ctx)
		if err != nil {
			return err
		}

		logger.InfoContext(ctx, fmt.Sprintf("Retention job took %f minutes to run", time.Since(c).Minutes()))
		return nil
	}
}
