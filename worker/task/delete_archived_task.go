package task

import (
	"context"
	"errors"
	"fmt"
	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/queue/redis"
	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v9"
	"github.com/hibiken/asynq"
	"time"
)

func DeleteArchivedTasks(r queue.Queuer, rd *rdb.Redis) func(context.Context, *asynq.Task) error {
	pool := goredis.NewPool(rd.Client())
	rs := redsync.New(pool)

	return func(ctx context.Context, t *asynq.Task) error {
		const mutexName = "convoy:delete_archived_tasks:mutex"
		mutex := rs.NewMutex(mutexName, redsync.WithExpiry(time.Second), redsync.WithTries(1))

		tctx, cancel := context.WithTimeout(ctx, time.Second*2)
		defer cancel()

		err := mutex.LockContext(tctx)
		if err != nil {
			return fmt.Errorf("failed to obtain lock: %v", err)
		}

		defer func() {
			tctx, cancel := context.WithTimeout(ctx, time.Second*2)
			defer cancel()

			// Release the lock so other processes or threads can obtain a lock.
			ok, err := mutex.UnlockContext(tctx)
			if !ok || err != nil {
				log.WithError(err).Error("failed to release lock")
			}
		}()

		queues := []string{
			string(convoy.EventQueue),
			string(convoy.CreateEventQueue),
			string(convoy.ScheduleQueue),
			string(convoy.DefaultQueue),
			string(convoy.StreamQueue),
			string(convoy.MetaEventQueue),
		}

		var q *redis.RedisQueue
		q, ok := r.(*redis.RedisQueue)
		if !ok {
			log.FromContext(ctx).WithError(err).Errorf("invalid queue type: %T", r)
			return errors.New("invalid queue type")
		}

		for _, qu := range queues {
			_, err := q.Inspector().DeleteAllArchivedTasks(qu)
			if err != nil {
				log.FromContext(ctx).WithError(err).Errorf("failed to delete archived task from queue - %s", qu)
				continue
			}
		}

		return nil
	}
}
