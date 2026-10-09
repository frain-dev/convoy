package worker

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/queue"
)

type schedulerStubQueue struct {
	opts     queue.QueueOptions
	writes   chan *queue.Job
	writeErr error
}

func (s schedulerStubQueue) Write(_ context.Context, _ convoy.TaskName, _ convoy.QueueName, job *queue.Job) error {
	if s.writes != nil {
		s.writes <- job
	}
	return s.writeErr
}

func (s schedulerStubQueue) WriteWithoutTimeout(context.Context, convoy.TaskName, convoy.QueueName, *queue.Job) error {
	return nil
}

func (s schedulerStubQueue) Options() queue.QueueOptions {
	return s.opts
}

func TestNewSchedulerPostgresEnqueuesCronTask(t *testing.T) {
	writes := make(chan *queue.Job, 1)
	s := NewPostgresScheduler(schedulerStubQueue{
		opts:   queue.QueueOptions{Type: queue.ProviderPostgres},
		writes: writes,
	}, log.New("test", log.LevelError)).(*postgresScheduler)
	s.RegisterTask("15 2 * * *", convoy.ScheduleQueue, convoy.SnapshotUsage)
	require.Len(t, s.cron.Entries(), 1)

	s.cron.Entries()[0].Job.Run()
	select {
	case job := <-writes:
		require.Contains(t, job.ID, queue.CronJobIDPrefix+string(convoy.SnapshotUsage)+":")
	case <-time.After(time.Second):
		t.Fatal("postgres cron task was not enqueued")
	}
}

// The asynq scheduler runs cron specs in UTC. Matching it keeps "0 1 * * *"
// meaning the same instant under both providers on a host in any timezone.
func TestNewSchedulerPostgresRunsCronInUTC(t *testing.T) {
	s := NewPostgresScheduler(schedulerStubQueue{}, log.New("test", log.LevelError)).(*postgresScheduler)
	require.Equal(t, time.UTC, s.cron.Location())
}

// Independent scheduler instances must derive the same job ID for one tick,
// because that ID is what lets the queue enqueue the tick exactly once.
func TestNewSchedulerPostgresTickIDIsStableAcrossInstances(t *testing.T) {
	tests := []struct {
		name       string
		instances  int
		concurrent bool
	}{
		{name: "concurrent replicas", instances: 4, concurrent: true},
		{name: "restart within the same tick", instances: 2, concurrent: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			writes := make(chan *queue.Job, tc.instances)
			run := func() {
				s := NewPostgresScheduler(schedulerStubQueue{writes: writes}, log.New("test", log.LevelError)).(*postgresScheduler)
				s.RegisterTask("15 2 * * *", convoy.ScheduleQueue, convoy.SnapshotUsage)
				s.cron.Entries()[0].Job.Run()
			}

			before := time.Now()
			if tc.concurrent {
				var wg sync.WaitGroup
				for range tc.instances {
					wg.Add(1)
					go func() {
						defer wg.Done()
						run()
					}()
				}
				wg.Wait()
			} else {
				for range tc.instances {
					run()
				}
			}
			after := time.Now()
			close(writes)

			firstTick := queue.CronJobID(convoy.SnapshotUsage, before)
			lastTick := queue.CronJobID(convoy.SnapshotUsage, after)

			ids := map[string]struct{}{}
			for job := range writes {
				require.Contains(t, []string{firstTick, lastTick}, job.ID)
				ids[job.ID] = struct{}{}
			}
			if firstTick == lastTick {
				require.Len(t, ids, 1, "every instance must enqueue the same tick")
			}
		})
	}
}

// Failure policy: an enqueue error is logged and the tick is dropped, matching
// the asynq scheduler. It must not panic or take the process down.
func TestNewSchedulerPostgresEnqueueErrorIsNotFatal(t *testing.T) {
	writes := make(chan *queue.Job, 1)
	s := NewPostgresScheduler(schedulerStubQueue{
		writes:   writes,
		writeErr: errors.New("queue unavailable"),
	}, log.New("test", log.LevelError)).(*postgresScheduler)
	s.RegisterTask("15 2 * * *", convoy.ScheduleQueue, convoy.SnapshotUsage)

	require.NotPanics(t, s.cron.Entries()[0].Job.Run)
	require.Len(t, writes, 1)
}
