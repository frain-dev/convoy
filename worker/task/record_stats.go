package task

import (
	"context"
	"fmt"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
	"github.com/frain-dev/convoy/internal/pkg/stats"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v9"
	"github.com/hibiken/asynq"
	"time"
)

func RecordStats(rd *rdb.Redis, deliveryRepo datastore.EventDeliveryRepository, configRepo datastore.ConfigurationRepository) func(context.Context, *asynq.Task) error {
	pool := goredis.NewPool(rd.Client())
	rs := redsync.New(pool)

	return func(ctx context.Context, t *asynq.Task) error {
		const mutexName = "convoy:record_stats:mutex"
		mutex := rs.NewMutex(mutexName, redsync.WithExpiry(time.Second), redsync.WithTries(1))

		tctx, cancel := context.WithTimeout(ctx, time.Second*2)
		defer cancel()

		err := mutex.LockContext(tctx)
		if err != nil {
			return fmt.Errorf("failed to obtain lock: %v", err)
		}

		defer func() {
			innerCtx, innerCancel := context.WithTimeout(ctx, time.Second*2)
			defer innerCancel()

			// Release the lock so other processes or threads can obtain a lock.
			ok, err := mutex.UnlockContext(innerCtx)
			if !ok || err != nil {
				log.WithError(err).Error("failed to release lock")
			}
		}()

		s := stats.NewStats(deliveryRepo, configRepo)
		err = s.Record(ctx)
		if err != nil {
			return err
		}

		return nil
	}
}
