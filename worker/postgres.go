package worker

import (
	"context"
	"maps"
	"strconv"
	"sync"
	"time"

	"github.com/hibiken/asynq"

	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/worker/task"
)

const (
	postgresReclaimInterval = 30 * time.Second

	// maxHeartbeatTimeout caps one renewal attempt so a stalled database cannot
	// hold the renewal loop for a long lease's whole interval.
	maxHeartbeatTimeout = 10 * time.Second

	// heartbeatsPerLease is how many renewals fit in one lease. Renewal must
	// stay well under the lease so a few slow or failed renewals cannot expire
	// a live worker's claim: at six, a worker misses five in a row before
	// another consumer may take its job. The interval is derived from the
	// lease rather than configured beside it so the two cannot drift.
	heartbeatsPerLease = 6
)

// minHeartbeatInterval keeps renewal traffic bounded if the lease is ever
// configured very low. It is a variable so tests can drive the renewal loop
// without waiting on wall clock.
var minHeartbeatInterval = time.Second

// heartbeatInterval derives the renewal period from the queue's lease.
func heartbeatInterval(lease time.Duration) time.Duration {
	interval := lease / heartbeatsPerLease
	if interval < minHeartbeatInterval {
		return minHeartbeatInterval
	}
	return interval
}

// heartbeatTimeout bounds one renewal attempt. It must not exceed the interval:
// the renewal loop calls Heartbeat inline and a ticker holds only one pending
// tick, so an attempt that outlives its interval stretches the real cadence to
// the timeout. At a 30s lease that turns a 5s interval into 10s attempts and
// leaves two tries before the lease expires instead of five, and a live worker's
// job gets reclaimed under it. Keeping the timeout at or below the interval
// holds the five-miss guarantee at every configurable lease.
func heartbeatTimeout(interval time.Duration) time.Duration {
	if interval < maxHeartbeatTimeout {
		return interval
	}
	return maxHeartbeatTimeout
}

// PostgresConsumerQueue is the worker-owned adapter contract for consuming Postgres
// queue jobs. Provider construction remains in the queue factory.
type PostgresConsumerQueue interface {
	Claim(context.Context, []string, int) ([]queue.ClaimedJob, error)
	Complete(context.Context, string) error
	Retry(context.Context, string, time.Time, bool, string) error
	Archive(context.Context, string, string) error
	Release(context.Context, []string) error
	ReclaimStuck(context.Context) (int64, error)
	Heartbeat(context.Context, []string) error
	LeaseTimeout() time.Duration
	ClaimBatchSize() int
	PollIdle() time.Duration
	// Wake is nil when LISTEN/NOTIFY is disabled.
	Wake() <-chan struct{}
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

	claimLimit int
	pollIdle   time.Duration

	tasks chan queue.ClaimedJob
	quit  chan struct{}

	// heldQuit outlives quit: claims stay leased until the last handler drains,
	// so renewal must not stop when polling does.
	heldQuit chan struct{}

	heldMu sync.Mutex
	held   map[string]struct{}

	// heartbeatEvery is derived from the queue's lease at construction.
	heartbeatEvery time.Duration

	pollerWg    sync.WaitGroup
	workerWg    sync.WaitGroup
	heartbeatWg sync.WaitGroup
	start       sync.Once
	stop        sync.Once
}

func (b *postgresConsumerBackend) newRunner(ctx context.Context, poolSize int, names map[string]int, mux *asynq.ServeMux, lo log.Logger, _ log.Level) (runner, error) {
	if poolSize <= 0 {
		poolSize = 1
	}
	claimBatchSize := b.queue.ClaimBatchSize()
	claimLimit := claimBatchSize
	if poolSize < claimLimit {
		claimLimit = poolSize
	}
	lo.Infof(
		"Postgres consumer configured with pool size %d and claim batch size %d (claim limit %d).",
		poolSize,
		claimBatchSize,
		claimLimit,
	)
	return &postgresRunner{
		ctx:            ctx,
		poolSize:       poolSize,
		queue:          b.queue,
		names:          names,
		mux:            mux,
		log:            lo,
		claimLimit:     claimLimit,
		pollIdle:       b.queue.PollIdle(),
		heartbeatEvery: heartbeatInterval(b.queue.LeaseTimeout()),
		tasks:          make(chan queue.ClaimedJob, poolSize),
		quit:           make(chan struct{}),
		heldQuit:       make(chan struct{}),
		held:           make(map[string]struct{}),
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
		r.heartbeatWg.Add(1)
		go r.heartbeat()
	})
	return nil
}

func (r *postgresRunner) Stop() {
	r.stop.Do(func() {
		close(r.quit)
		r.pollerWg.Wait()
		close(r.tasks)
		r.workerWg.Wait()
		close(r.heldQuit)
		r.heartbeatWg.Wait()
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
	limit := r.claimLimit

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
			// Hold the claim before handing it off. A job buffered in the
			// channel is already claimed, so its lease is running whether or not
			// a worker has picked it up yet.
			r.hold(job.ID)
			select {
			case <-r.quit:
				for _, leftover := range jobs[i:] {
					unsent = append(unsent, leftover.ID)
					r.release(leftover.ID)
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

func (r *postgresRunner) hold(id string) {
	r.heldMu.Lock()
	r.held[id] = struct{}{}
	r.heldMu.Unlock()
}

func (r *postgresRunner) release(id string) {
	r.heldMu.Lock()
	delete(r.held, id)
	r.heldMu.Unlock()
}

func (r *postgresRunner) heldIDs() []string {
	r.heldMu.Lock()
	defer r.heldMu.Unlock()
	if len(r.held) == 0 {
		return nil
	}
	ids := make([]string, 0, len(r.held))
	for id := range r.held {
		ids = append(ids, id)
	}
	return ids
}

// heartbeat renews the lease on every claim this consumer holds, so a job is
// reclaimed because its worker died, not because its handler is slow.
func (r *postgresRunner) heartbeat() {
	defer r.heartbeatWg.Done()

	ticker := time.NewTicker(r.heartbeatEvery)
	defer ticker.Stop()

	for {
		select {
		case <-r.heldQuit:
			return
		case <-ticker.C:
		}

		ids := r.heldIDs()
		if len(ids) == 0 {
			continue
		}
		// Renewal must outlive a cancelled consumer context: claims are still
		// held while in-flight handlers drain.
		ctx, cancel := context.WithTimeout(context.WithoutCancel(r.ctx), heartbeatTimeout(r.heartbeatEvery))
		if err := r.queue.Heartbeat(ctx, ids); err != nil {
			r.log.Error("postgres queue heartbeat failed", "error", err, "jobs", len(ids))
		}
		cancel()
	}
}

func (r *postgresRunner) reclaimStuck() {
	if _, err := r.queue.ReclaimStuck(r.ctx); err != nil {
		r.log.Error("postgres queue stuck reclaim failed", "error", err)
	}
}

func (r *postgresRunner) idle() {
	timer := time.NewTimer(r.pollIdle)
	defer timer.Stop()
	wake := r.queue.Wake()
	if wake == nil {
		select {
		case <-r.quit:
		case <-timer.C:
		}
		return
	}
	select {
	case <-r.quit:
	case <-wake:
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
	defer r.release(job.ID)

	headers := job.Headers
	if headers == nil {
		headers = make(map[string]string, 1)
	} else {
		headers = maps.Clone(headers)
	}
	headers[task.HeaderRetryCount] = strconv.Itoa(job.RetryCount)
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

// isCountedFailure decides whether a handler error counts against a job's retry
// budget. Rate limiting and an open circuit breaker are backpressure, not a
// failed attempt, so they must not consume retries or archive the job. Both
// backends share this: the redis runner passes it to asynq as IsFailure.
func isCountedFailure(err error) bool {
	if _, ok := err.(*task.RateLimitError); ok {
		return false
	}
	if _, ok := err.(*task.CircuitBreakerError); ok {
		return false
	}
	return true
}
