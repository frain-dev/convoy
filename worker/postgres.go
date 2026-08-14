package worker

import (
	"context"
	"sync"
	"time"

	"github.com/hibiken/asynq"

	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/worker/task"
)

const (
	postgresPollIdle        = 5 * time.Millisecond
	postgresClaimSize       = 64
	postgresReclaimInterval = 30 * time.Second
)

// PostgresConsumerQueue is the worker-owned adapter contract for consuming Postgres
// queue jobs. Provider construction remains in the queue factory.
type PostgresConsumerQueue interface {
	Claim(context.Context, []string, int) ([]queue.ClaimedJob, error)
	Complete(context.Context, string) error
	Retry(context.Context, string, time.Time, bool, string) error
	Archive(context.Context, string, string) error
	Release(context.Context, []string) error
	ReclaimStuck(context.Context) (int64, error)
}

type postgresConsumerBackend struct {
	queue PostgresConsumerQueue
}

type postgresRunner struct {
	ctx      context.Context
	poolSize int
	queue    PostgresConsumerQueue
	names    map[string]int
	mux      *asynq.ServeMux
	log      log.Logger

	tasks chan queue.ClaimedJob
	quit  chan struct{}

	pollerWg sync.WaitGroup
	workerWg sync.WaitGroup
	start    sync.Once
	stop     sync.Once
}

func (b *postgresConsumerBackend) newRunner(ctx context.Context, poolSize int, names map[string]int, mux *asynq.ServeMux, lo log.Logger, _ log.Level) (runner, error) {
	if poolSize <= 0 {
		poolSize = 1
	}
	return &postgresRunner{
		ctx:      ctx,
		poolSize: poolSize,
		queue:    b.queue,
		names:    names,
		mux:      mux,
		log:      lo,
		tasks:    make(chan queue.ClaimedJob, poolSize),
		quit:     make(chan struct{}),
	}, nil
}

func (r *postgresRunner) Start() error {
	r.start.Do(func() {
		for i := 0; i < r.poolSize; i++ {
			r.workerWg.Add(1)
			go r.worker()
		}
		r.pollerWg.Add(1)
		go r.poll()
	})
	return nil
}

func (r *postgresRunner) Stop() {
	r.stop.Do(func() {
		close(r.quit)
		r.pollerWg.Wait()
		close(r.tasks)
		r.workerWg.Wait()
	})
}

func (r *postgresRunner) poll() {
	defer r.pollerWg.Done()

	names := make([]string, 0, len(r.names))
	for name := range r.names {
		names = append(names, name)
	}
	priorities := queue.PriorityCycle(r.names)
	priorityIndex := 0
	limit := postgresClaimSize
	if r.poolSize < limit {
		limit = r.poolSize
	}

	reclaimTicker := time.NewTicker(postgresReclaimInterval)
	defer reclaimTicker.Stop()
	r.reclaimStuck()

	for {
		select {
		case <-r.quit:
			return
		case <-reclaimTicker.C:
			r.reclaimStuck()
		default:
		}

		claimNames := names
		if len(priorities) > 0 {
			claimNames = prioritizeQueue(names, priorities[priorityIndex])
			priorityIndex = (priorityIndex + 1) % len(priorities)
		}
		jobs, err := r.queue.Claim(r.ctx, claimNames, limit)
		if err != nil {
			r.log.Error("postgres queue claim failed", "error", err)
			r.idle()
			continue
		}
		if len(jobs) == 0 {
			r.idle()
			continue
		}

		var unsent []string
		for i, job := range jobs {
			select {
			case <-r.quit:
				for _, leftover := range jobs[i:] {
					unsent = append(unsent, leftover.ID)
				}
				if err := r.queue.Release(context.WithoutCancel(r.ctx), unsent); err != nil {
					r.log.Error("postgres queue release on stop failed", "error", err)
				}
				return
			case r.tasks <- job:
			}
		}
	}
}

func prioritizeQueue(names []string, preferred string) []string {
	ordered := make([]string, 0, len(names))
	if preferred != "" {
		ordered = append(ordered, preferred)
	}
	for _, name := range names {
		if name != preferred {
			ordered = append(ordered, name)
		}
	}
	return ordered
}

func (r *postgresRunner) reclaimStuck() {
	if _, err := r.queue.ReclaimStuck(r.ctx); err != nil {
		r.log.Error("postgres queue stuck reclaim failed", "error", err)
	}
}

func (r *postgresRunner) idle() {
	timer := time.NewTimer(postgresPollIdle)
	defer timer.Stop()
	select {
	case <-r.quit:
	case <-timer.C:
	}
}

func (r *postgresRunner) worker() {
	defer r.workerWg.Done()
	for job := range r.tasks {
		r.process(job)
	}
}

func (r *postgresRunner) process(job queue.ClaimedJob) {
	headers := job.Headers
	t := asynq.NewTaskWithHeaders(job.TaskName, job.Payload, headers)
	err := r.mux.ProcessTask(r.ctx, t)
	ctx := context.WithoutCancel(r.ctx)
	if err == nil {
		if completeErr := r.queue.Complete(ctx, job.ID); completeErr != nil {
			r.log.Error("postgres queue complete failed", "error", completeErr, "job", job.ID)
		}
		return
	}

	delay := task.GetRetryDelay(job.RetryCount+1, err, t)
	runAt := time.Now().Add(delay)
	msg := err.Error()

	if !isCountedFailure(err) {
		if retryErr := r.queue.Retry(ctx, job.ID, runAt, false, msg); retryErr != nil {
			r.log.Error("postgres queue retry failed", "error", retryErr, "job", job.ID)
		}
		return
	}

	if job.RetryCount >= job.MaxRetry {
		if archiveErr := r.queue.Archive(ctx, job.ID, msg); archiveErr != nil {
			r.log.Error("postgres queue archive failed", "error", archiveErr, "job", job.ID)
		}
		return
	}

	if retryErr := r.queue.Retry(ctx, job.ID, runAt, true, msg); retryErr != nil {
		r.log.Error("postgres queue retry failed", "error", retryErr, "job", job.ID)
	}
}

func isCountedFailure(err error) bool {
	if _, ok := err.(*task.RateLimitError); ok {
		return false
	}
	if _, ok := err.(*task.CircuitBreakerError); ok {
		return false
	}
	return true
}
