package task

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v9"
	"github.com/olamilekan000/surge/surge/job"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/queue/redis"
)

func DeleteArchivedTasks(r queue.Queuer, rd *rdb.Redis) func(context.Context, *job.JobEnvelope) error {
	pool := goredis.NewPool(rd.Client())
	rs := redsync.New(pool)

	return func(ctx context.Context, jobEnvelope *job.JobEnvelope) error {
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
			string(convoy.EventWorkflowQueue),
		}

		var q *redis.RedisQueue
		q, ok := r.(*redis.RedisQueue)
		if !ok {
			log.FromContext(ctx).WithError(err).Errorf("invalid queue type: %T", r)
			return errors.New("invalid queue type")
		}

		backend := q.Inspector()
		namespaces, err := backend.GetNamespaces(ctx)
		if err != nil {
			log.FromContext(ctx).WithError(err).Error("failed to get namespaces")
			namespaces = []string{"system"}
		}

		for _, ns := range namespaces {
			for _, qu := range queues {
				_, err := backend.Drain(ctx, ns, qu)
				if err != nil {
					log.FromContext(ctx).WithError(err).Errorf("failed to drain queue - %s in namespace %s", qu, ns)
					continue
				}
			}
		}

		return nil
	}
}
