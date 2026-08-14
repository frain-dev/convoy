package worker

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/queue"
)

type schedulerStubQueue struct {
	opts   queue.QueueOptions
	writes chan *queue.Job
}

func (s schedulerStubQueue) Write(_ context.Context, _ convoy.TaskName, _ convoy.QueueName, job *queue.Job) error {
	if s.writes != nil {
		s.writes <- job
	}
	return nil
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
		require.Contains(t, job.ID, "cron:"+string(convoy.SnapshotUsage)+":")
	case <-time.After(time.Second):
		t.Fatal("postgres cron task was not enqueued")
	}
}
