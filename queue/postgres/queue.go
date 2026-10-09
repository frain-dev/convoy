package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/internal/pkg/queue/tracectx"
	"github.com/frain-dev/convoy/queue"
)

var (
	_ queue.Queuer    = (*PostgresQueue)(nil)
	_ queue.Archiver  = (*PostgresQueue)(nil)
	_ queue.Inspector = (*PostgresQueue)(nil)

	// ErrTaskNotFound is returned by LastTaskError when no row matches.
	// Failure policy: fail closed — unknown is not "no prior error".
	ErrTaskNotFound = queue.ErrTaskNotFound
)

const (
	statusPending    = "pending"
	statusProcessing = "processing"
	statusArchived   = "archived"
	statusCompleted  = "completed"

	postgresBatchTimeout = 30 * time.Second
	postgresWeightDepth  = 8

	// Defaults for queue.PostgresTuning, applied when a field is left zero.
	// A single flusher caps enqueue throughput at one batch per statement round
	// trip no matter how fast the statement is, so callers queue behind it under
	// load; several flushers keep that many inserts moving across separate pool
	// connections while still leaving most of the pool for reads. Config
	// validation keeps write concurrency below the pool size.
	defaultBatchSize        = 64
	defaultBatchWait        = 2 * time.Millisecond
	defaultWriteConcurrency = 8
	defaultClaimBatchSize   = 64
	defaultPollIdle         = 5 * time.Millisecond

	// defaultMaxRetry matches asynq's unexported DefaultMaxRetry.
	defaultMaxRetry = 25

	// defaultStuckTimeout is the claim lease. Consumers renew it while a handler
	// runs, so it bounds how long a crashed worker's job stays unavailable
	// rather than how long a handler may take. Failure policy: fail open, a
	// job whose owner stopped renewing is returned to the queue. Keep this
	// several heartbeat intervals wide so a slow database cannot expire the
	// lease of a worker that is still alive and hand its job to a second one.
	defaultStuckTimeout = 90 * time.Second

	// cronJobPrefix is the marker the scheduler writes into cron job IDs.
	cronJobPrefix = queue.CronJobIDPrefix

	// cronTombstoneRetention keeps a finished cron row addressable long
	// enough that the archived-jobs cleanup cannot erase the tick a replica
	// with a lagging clock is still about to enqueue. Cron IDs carry an
	// absolute minute, so retaining them never blocks a later tick.
	cronTombstoneRetention = 24 * time.Hour
)

var (
	errMissingDB     = errors.New("database is required")
	errQueueClosed   = errors.New("queue is closed")
	ErrJobProcessing = errors.New("queue job is already processing")
)

const writeJobSQL = `
	INSERT INTO convoy.queue_jobs (
		id, task_name, queue_name, payload, headers,
		max_retry, retry_count, status, run_at, claimed_at, last_error, created_at, updated_at
	) VALUES ($1, $2, $3, $4, $5, $6, 0, $7, NOW() + make_interval(secs => $8), NULL, NULL, NOW(), NOW())
	ON CONFLICT (id) DO UPDATE SET
		task_name = EXCLUDED.task_name,
		queue_name = EXCLUDED.queue_name,
		payload = EXCLUDED.payload,
		headers = EXCLUDED.headers,
		max_retry = EXCLUDED.max_retry,
		retry_count = 0,
		status = EXCLUDED.status,
		run_at = EXCLUDED.run_at,
		claimed_at = NULL,
		claim_id = NULL,
		last_error = NULL,
		updated_at = NOW()
	WHERE (
		convoy.queue_jobs.status <> $9
		OR (convoy.queue_jobs.status = $9
		    AND convoy.queue_jobs.queue_name = $13
		    AND EXCLUDED.queue_name = $14)
	)
	  AND NOT (
		convoy.queue_jobs.id LIKE $10
		AND convoy.queue_jobs.status IN ($11, $12)
	  )`

// writeJobsSQL is the batch form of writeJobSQL: one statement for the whole
// flush window instead of one per job. Conflict handling and the cron guard are
// identical; RETURNING id reports which rows the guard let through, which is how
// per-job results survive the collapse into a single round trip.
const writeJobsSQL = `
	INSERT INTO convoy.queue_jobs (
		id, task_name, queue_name, payload, headers,
		max_retry, retry_count, status, run_at, claimed_at, last_error, created_at, updated_at
	)
	SELECT j.id, j.task_name, j.queue_name, j.payload, j.headers,
	       j.max_retry, 0, $8, NOW() + make_interval(secs => j.delay), NULL, NULL, NOW(), NOW()
	FROM UNNEST(
		$1::text[], $2::text[], $3::text[], $4::bytea[],
		$5::jsonb[], $6::int[], $7::double precision[]
	) AS j(id, task_name, queue_name, payload, headers, max_retry, delay)
	ON CONFLICT (id) DO UPDATE SET
		task_name = EXCLUDED.task_name,
		queue_name = EXCLUDED.queue_name,
		payload = EXCLUDED.payload,
		headers = EXCLUDED.headers,
		max_retry = EXCLUDED.max_retry,
		retry_count = 0,
		status = EXCLUDED.status,
		run_at = EXCLUDED.run_at,
		claimed_at = NULL,
		claim_id = NULL,
		last_error = NULL,
		updated_at = NOW()
	WHERE (
		convoy.queue_jobs.status <> $9
		OR (convoy.queue_jobs.status = $9
		    AND convoy.queue_jobs.queue_name = $13
		    AND EXCLUDED.queue_name = $14)
	)
	  AND NOT (
		convoy.queue_jobs.id LIKE $10
		AND convoy.queue_jobs.status IN ($11, $12)
	  )
	RETURNING id`

// PostgresQueue implements queue.Queuer with convoy.queue_jobs as the broker.
type PostgresQueue struct {
	db             *sqlx.DB
	opts           queue.QueueOptions
	stuckTimeout   time.Duration
	batchSize      int
	batchWait      time.Duration
	claimBatchSize int
	pollIdle       time.Duration
	writes         chan writeRequest
	completions    chan completeRequest

	// quit asks the batchers to stop; done is closed once every one of them has
	// returned. A caller that has already been admitted to a channel waits on
	// done as well as its own result, because after the batchers are joined
	// nothing is left to answer it.
	quit      chan struct{}
	done      chan struct{}
	quitOnce  sync.Once
	batcherWg sync.WaitGroup

	wake          chan struct{}
	listenerQuit  chan struct{}
	listenerDone  chan struct{}
	notifyEnabled bool

	// leases is this process's claim_id for each job it Claim'd. Heartbeat
	// pairs those uuids with the row; a later Claim overwrites claim_id, so a
	// stale worker cannot renew the new owner's lease. Generation does not
	// change on renew: claimed_at is what Heartbeat writes, and cannot be the
	// CAS key.
	leaseMu sync.Mutex
	leases  map[string]string
}

type writeRequest struct {
	id        string
	taskName  string
	queueName string
	payload   []byte
	headers   []byte
	maxRetry  int
	delay     float64
	result    chan error
}

type completeRequest struct {
	id     string
	result chan error
}

// withDefaults fills every field the caller left zero with the package default.
func withDefaults(t queue.PostgresTuning) queue.PostgresTuning {
	if t.BatchSize < 1 {
		t.BatchSize = defaultBatchSize
	}
	if t.BatchWait < 1 {
		t.BatchWait = defaultBatchWait
	}
	if t.WriteConcurrency < 1 {
		t.WriteConcurrency = defaultWriteConcurrency
	}
	if t.LeaseTimeout < 1 {
		t.LeaseTimeout = defaultStuckTimeout
	}
	if t.ClaimBatchSize < 1 {
		t.ClaimBatchSize = defaultClaimBatchSize
	}
	if t.PollIdle < 1 {
		t.PollIdle = defaultPollIdle
	}
	return t
}

func NewQueue(opts queue.QueueOptions) (*PostgresQueue, error) {
	if opts.DB == nil {
		return nil, errMissingDB
	}
	t := withDefaults(opts.PostgresTuning)
	q := &PostgresQueue{
		db:             opts.DB,
		opts:           opts,
		stuckTimeout:   t.LeaseTimeout,
		batchSize:      t.BatchSize,
		batchWait:      t.BatchWait,
		claimBatchSize: t.ClaimBatchSize,
		pollIdle:       t.PollIdle,
		writes:         make(chan writeRequest, t.BatchSize*t.WriteConcurrency),
		completions:    make(chan completeRequest, t.BatchSize),
		quit:           make(chan struct{}),
		done:           make(chan struct{}),
		leases:         make(map[string]string),
	}
	q.batcherWg.Add(t.WriteConcurrency + 1)
	for i := 0; i < t.WriteConcurrency; i++ {
		go q.runWriteBatcher()
	}
	go q.runCompleteBatcher()
	if opts.PostgresConnString != "" {
		q.wake = make(chan struct{}, 1)
		q.listenerQuit = make(chan struct{})
		q.listenerDone = make(chan struct{})
		q.notifyEnabled = true
		go q.runListener(opts.PostgresConnString)
	}
	return q, nil
}

// Close stops the batcher goroutines and waits for them to return. A long-lived
// process can skip it, but anything that builds a queue per unit of work leaks
// writeConcurrency+1 goroutines per build without it, each holding the pool.
// Writes admitted before Close still commit; writes offered after it are
// rejected rather than parked on a channel nobody is draining.
func (q *PostgresQueue) Close() error {
	q.quitOnce.Do(func() {
		if q.listenerQuit != nil {
			close(q.listenerQuit)
			<-q.listenerDone
		}
		close(q.quit)
		q.batcherWg.Wait()
		close(q.done)
	})
	<-q.done
	return nil
}

// awaitResult waits for the batcher's answer, or gives up once every batcher
// has returned. Checking result again under done matters: a batcher answers its
// whole batch before exiting, so a request that did commit has its result
// buffered and must not be reported as rejected.
func awaitResult(result <-chan error, done <-chan struct{}) error {
	select {
	case err := <-result:
		return err
	case <-done:
		select {
		case err := <-result:
			return err
		default:
			return errQueueClosed
		}
	}
}

func (q *PostgresQueue) Options() queue.QueueOptions {
	return q.opts
}

func (q *PostgresQueue) SetStuckTimeout(d time.Duration) {
	q.stuckTimeout = d
}

// LeaseTimeout is how long a claim survives without renewal. Consumers derive
// their renewal interval from it so the two cannot be configured into a state
// where a live worker's lease expires under it.
func (q *PostgresQueue) LeaseTimeout() time.Duration {
	return q.stuckTimeout
}

// ClaimBatchSize is how many jobs one Claim round trip may take.
func (q *PostgresQueue) ClaimBatchSize() int {
	return q.claimBatchSize
}

// PollIdle is how long the consumer waits when Claim returns no work.
func (q *PostgresQueue) PollIdle() time.Duration {
	return q.pollIdle
}

func (q *PostgresQueue) Write(ctx context.Context, taskName convoy.TaskName, queueName convoy.QueueName, job *queue.Job) error {
	return q.write(ctx, taskName, queueName, job)
}

func (q *PostgresQueue) WriteWithoutTimeout(ctx context.Context, taskName convoy.TaskName, queueName convoy.QueueName, job *queue.Job) error {
	// asynq Timeout(0) is a worker-side deadline. Postgres has no per-task
	// timeout column; handlers use the consumer context the same way.
	return q.write(ctx, taskName, queueName, job)
}

func (q *PostgresQueue) write(ctx context.Context, taskName convoy.TaskName, queueName convoy.QueueName, job *queue.Job) error {
	if job.ID == "" {
		job.ID = ulid.Make().String()
	}
	tracectx.InjectIntoJob(ctx, job)

	headers := job.Headers
	if headers == nil {
		headers = map[string]string{}
	}
	headerBytes, err := json.Marshal(headers)
	if err != nil {
		return fmt.Errorf("marshal job headers: %w", err)
	}

	maxRetry := defaultMaxRetry
	if job.MaxRetry != nil {
		maxRetry = *job.MaxRetry
	}

	payload := job.Payload
	if payload == nil {
		payload = []byte{}
	}

	delay := job.Delay
	if delay < 0 {
		delay = 0
	}

	req := writeRequest{
		id:        job.ID,
		taskName:  string(taskName),
		queueName: string(queueName),
		payload:   payload,
		headers:   headerBytes,
		maxRetry:  maxRetry,
		delay:     delay.Seconds(),
		result:    make(chan error, 1),
	}
	// Refuse before offering. Both cases below can be ready at once after Close
	// and select would pick either, so without this a write can still land in a
	// buffer no batcher is draining.
	select {
	case <-q.quit:
		return errQueueClosed
	default:
	}

	select {
	case q.writes <- req:
	case <-q.quit:
		return errQueueClosed
	case <-ctx.Done():
		return ctx.Err()
	}

	// Failure policy: after admission, wait for the durable commit even if the
	// caller cancels. Returning early could let a parent job complete before
	// its child is durable; deterministic job IDs make an ambiguous retry safe.
	return awaitResult(req.result, q.done)
}

// collectBatch fills a flush window that already holds first, taking at most
// batchSize requests and waiting no longer than batchWait for the rest.
func collectBatch[T any](first T, ch <-chan T, batchSize int, batchWait time.Duration) []T {
	batch := []T{first}
	timer := time.NewTimer(batchWait)
	defer func() {
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
	}()

	for len(batch) < batchSize {
		select {
		case req := <-ch:
			batch = append(batch, req)
		case <-timer.C:
			return batch
		}
	}
	return batch
}

func (q *PostgresQueue) runWriteBatcher() {
	defer q.batcherWg.Done()

	for {
		var first writeRequest
		select {
		case first = <-q.writes:
		case <-q.quit:
			// Drain what was already admitted before giving up: those callers
			// are blocked on their result channel and a durable commit was
			// promised to them.
			q.drainWrites()
			return
		}

		batch := collectBatch(first, q.writes, q.batchSize, q.batchWait)

		results := q.writeBatch(batch)
		for i := range batch {
			batch[i].result <- results[i]
		}
	}
}

// drainWrites commits whatever is still buffered after Close. Several batchers
// may run this at once; each takes what it can get and stops when the channel
// is empty.
func (q *PostgresQueue) drainWrites() {
	for {
		var batch []writeRequest
		for len(batch) < q.batchSize {
			select {
			case req := <-q.writes:
				batch = append(batch, req)
				continue
			default:
			}
			break
		}
		if len(batch) == 0 {
			return
		}
		results := q.writeBatch(batch)
		for i := range batch {
			batch[i].result <- results[i]
		}
	}
}

func (q *PostgresQueue) writeBatch(batch []writeRequest) []error {
	ctx, cancel := context.WithTimeout(context.Background(), postgresBatchTimeout)
	defer cancel()

	results := make([]error, len(batch))
	if len(batch) == 1 {
		err := q.execWrite(ctx, q.db, batch[0])
		results[0] = err
		if err == nil {
			q.notifyPending()
		}
		return results
	}

	// ON CONFLICT DO UPDATE cannot touch the same row twice in one command, so a
	// window holding the same job ID twice collapses to the last request. That
	// matches the sequential path, where the later write overwrote the earlier one.
	last := make(map[string]int, len(batch))
	for i := range batch {
		last[batch[i].id] = i
	}

	// Flush windows run concurrently and ON CONFLICT takes row locks in
	// statement order, so two windows holding the same pair of job IDs in
	// opposite orders would deadlock against each other. Sorting by ID gives
	// every window the same lock order.
	order := make([]int, 0, len(last))
	for i := range batch {
		if last[batch[i].id] == i {
			order = append(order, i)
		}
	}
	sort.Slice(order, func(a, b int) bool { return batch[order[a]].id < batch[order[b]].id })

	n := len(order)
	ids := make([]string, 0, n)
	taskNames := make([]string, 0, n)
	queueNames := make([]string, 0, n)
	payloads := make([][]byte, 0, n)
	headers := make([]string, 0, n)
	maxRetries := make([]int64, 0, n)
	delays := make([]float64, 0, n)
	for _, i := range order {
		ids = append(ids, batch[i].id)
		taskNames = append(taskNames, batch[i].taskName)
		queueNames = append(queueNames, batch[i].queueName)
		payloads = append(payloads, batch[i].payload)
		headers = append(headers, string(batch[i].headers))
		maxRetries = append(maxRetries, int64(batch[i].maxRetry))
		delays = append(delays, batch[i].delay)
	}

	rows, err := q.db.QueryContext(ctx, writeJobsSQL,
		pq.StringArray(ids), pq.StringArray(taskNames), pq.StringArray(queueNames),
		pq.ByteaArray(payloads), pq.StringArray(headers), pq.Int64Array(maxRetries),
		pq.Float64Array(delays), statusPending, statusProcessing,
		cronJobPrefix+"%", statusArchived, statusCompleted,
		string(convoy.EventQueue), string(convoy.RetryEventQueue),
	)
	if err != nil {
		return fillErrors(results, err)
	}
	written := make(map[string]struct{}, n)
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			_ = rows.Close()
			return fillErrors(results, err)
		}
		written[id] = struct{}{}
	}
	if err = rows.Err(); err != nil {
		_ = rows.Close()
		return fillErrors(results, err)
	}
	if err = rows.Close(); err != nil {
		return fillErrors(results, err)
	}

	for i := range batch {
		if _, ok := written[batch[i].id]; ok {
			continue
		}
		results[i] = skippedWriteResult(batch[i].id)
	}
	if len(written) > 0 {
		q.notifyPending()
	}
	return results
}

// skippedWriteResult explains a row the conflict guard rejected. A cron tick
// that already fired is expected and idempotent; anything else is a job whose
// active claim must resolve before it can be re-enqueued on the same queue.
// Cross-queue writes while processing are allowed: that is the EventQueue to
// RetryEventQueue handoff ProcessEventDelivery performs in its defer.
func skippedWriteResult(id string) error {
	if len(id) >= len(cronJobPrefix) && id[:len(cronJobPrefix)] == cronJobPrefix {
		return nil
	}
	return ErrJobProcessing
}

func fillErrors(results []error, err error) []error {
	for i := range results {
		results[i] = err
	}
	return results
}

func (q *PostgresQueue) execWrite(ctx context.Context, db sqlx.ExecerContext, req writeRequest) error {
	// run_at is PostgreSQL NOW() so Claim's run_at <= NOW() uses one clock.
	res, err := db.ExecContext(ctx, writeJobSQL,
		req.id, req.taskName, req.queueName, req.payload, req.headers,
		req.maxRetry, statusPending, req.delay, statusProcessing,
		cronJobPrefix+"%", statusArchived, statusCompleted,
		string(convoy.EventQueue), string(convoy.RetryEventQueue),
	)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		// Failure policy: an in-flight task is not replaced or reported as
		// re-enqueued. The caller must retry after the active claim resolves.
		return skippedWriteResult(req.id)
	}
	return nil
}

func (q *PostgresQueue) Claim(ctx context.Context, queueNames []string, limit int) ([]queue.ClaimedJob, error) {
	if limit <= 0 || len(queueNames) == 0 {
		return nil, nil
	}

	rows, err := q.db.QueryxContext(ctx, `
		WITH candidates AS MATERIALIZED (
			SELECT id, queue_name, run_at
			FROM convoy.queue_jobs
			WHERE status = $4
			  AND run_at <= NOW()
			  AND queue_name = ANY($1)
			  -- A paused queue keeps its work; the workers just stop taking it.
			  -- The gate is here rather than in the runner's queue list so it
			  -- applies to every replica the moment it is set, with no cache to
			  -- go stale. queue_state holds a row only per queue an operator has
			  -- touched, so this anti-join is against a table of a few rows.
			  AND NOT EXISTS (
				SELECT 1 FROM convoy.queue_state s
				WHERE s.queue_name = convoy.queue_jobs.queue_name
				  AND s.paused_at IS NOT NULL
			  )
			ORDER BY run_at ASC
			LIMIT ($2::integer * $5::integer + 1)
			FOR UPDATE SKIP LOCKED
		),
		claim AS (
			SELECT id
			FROM candidates
			ORDER BY
				CASE
					WHEN (SELECT COUNT(*) FROM candidates) > ($2::integer * $5::integer)
					THEN array_position($1::text[], queue_name)
					ELSE 0
				END,
				run_at ASC
			LIMIT $2
		)
		UPDATE convoy.queue_jobs AS j
		SET status = $3,
		    claimed_at = NOW(),
		    claim_id = gen_random_uuid(),
		    updated_at = NOW()
		FROM claim
		WHERE j.id = claim.id
		RETURNING j.id, j.task_name, j.queue_name, j.payload, j.headers, j.max_retry, j.retry_count, j.claim_id::text`,
		pq.Array(queueNames), limit, statusProcessing, statusPending, postgresWeightDepth,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []queue.ClaimedJob
	claimIDs := make([]string, 0)
	for rows.Next() {
		var (
			job         queue.ClaimedJob
			headerBytes []byte
			claimID     string
		)
		if err := rows.Scan(&job.ID, &job.TaskName, &job.QueueName, &job.Payload, &headerBytes, &job.MaxRetry, &job.RetryCount, &claimID); err != nil {
			return nil, err
		}
		if len(headerBytes) > 0 {
			if err := json.Unmarshal(headerBytes, &job.Headers); err != nil {
				return nil, fmt.Errorf("unmarshal job headers: %w", err)
			}
		}
		jobs = append(jobs, job)
		claimIDs = append(claimIDs, claimID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	q.rememberClaims(jobs, claimIDs)
	return jobs, nil
}

func (q *PostgresQueue) Complete(ctx context.Context, id string) error {
	req := completeRequest{id: id, result: make(chan error, 1)}
	select {
	case <-q.quit:
		return errQueueClosed
	default:
	}

	select {
	case q.completions <- req:
	case <-q.quit:
		return errQueueClosed
	case <-ctx.Done():
		return ctx.Err()
	}
	return awaitResult(req.result, q.done)
}

func (q *PostgresQueue) runCompleteBatcher() {
	defer q.batcherWg.Done()

	for {
		var first completeRequest
		select {
		case first = <-q.completions:
		case <-q.quit:
			// Drain what was already admitted before giving up: those callers
			// are blocked on their result channel and a durable completion was
			// promised to them.
			q.drainCompletions()
			return
		}

		batch := collectBatch(first, q.completions, q.batchSize, q.batchWait)

		ids := make([]string, len(batch))
		for i := range batch {
			ids[i] = batch[i].id
		}
		err := q.completeBatch(ids)
		for i := range batch {
			batch[i].result <- err
		}
	}
}

// drainCompletions finishes whatever is still buffered after Close. Several
// batchers may run this at once; each takes what it can get and stops when the
// channel is empty.
func (q *PostgresQueue) drainCompletions() {
	for {
		var batch []completeRequest
		for len(batch) < q.batchSize {
			select {
			case req := <-q.completions:
				batch = append(batch, req)
				continue
			default:
			}
			break
		}
		if len(batch) == 0 {
			return
		}
		ids := make([]string, len(batch))
		for i := range batch {
			ids[i] = batch[i].id
		}
		err := q.completeBatch(ids)
		for i := range batch {
			batch[i].result <- err
		}
	}
}

func (q *PostgresQueue) completeBatch(ids []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), postgresBatchTimeout)
	defer cancel()

	// A successful task's row is deleted, so this statement is the only place
	// that ever sees it finish. The throughput counter is folded into the same
	// round trip rather than issued after it: a separate statement would double
	// the cost of the hottest write in the driver, and could be lost between
	// the two, undercounting exactly when the queue is busiest.
	_, err := q.db.ExecContext(ctx, `
		WITH completed_cron AS (
			UPDATE convoy.queue_jobs
			SET status = $3,
			    claimed_at = NULL,
			    updated_at = NOW()
			WHERE id = ANY($1)
			  AND status = $2
			  AND id LIKE $4
			RETURNING queue_name
		),
		deleted AS (
			DELETE FROM convoy.queue_jobs
			WHERE id = ANY($1)
			  AND status = $2
			  AND id NOT LIKE $4
			RETURNING queue_name
		),
		finished AS (
			SELECT queue_name, COUNT(*) AS total
			FROM (
				SELECT queue_name FROM completed_cron
				UNION ALL
				SELECT queue_name FROM deleted
			) rows
			GROUP BY queue_name
		)
		INSERT INTO convoy.queue_job_stats AS s (queue_name, day, processed)
		SELECT queue_name, (NOW() AT TIME ZONE 'UTC')::date, total FROM finished
		ON CONFLICT (queue_name, day) DO UPDATE
		SET processed = s.processed + EXCLUDED.processed,
		    updated_at = NOW()`,
		pq.Array(ids), statusProcessing, statusCompleted, cronJobPrefix+"%",
	)
	if err != nil {
		return err
	}
	q.dropLeases(ids)
	return nil
}

// LastTaskError returns the persisted last_error for a job, matching the
// string Retry/Archive write. Empty string means no prior error. Missing
// rows and transport failures return an error (not empty).
func (q *PostgresQueue) LastTaskError(queueName, jobID string) (string, error) {
	var lastError sql.NullString
	err := q.db.QueryRow(`
		SELECT last_error
		FROM convoy.queue_jobs
		WHERE queue_name = $1 AND id = $2`,
		queueName, jobID,
	).Scan(&lastError)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrTaskNotFound
		}
		return "", err
	}
	if !lastError.Valid {
		return "", nil
	}
	return lastError.String, nil
}

// The two retry statements differ only in retry_count. Both snap "now"/past Go
// times onto PostgreSQL NOW() so Claim's run_at <= NOW() cannot miss an
// immediate retry when the two clocks disagree by milliseconds.
//
// Only the counted statement records a failure. An uncounted retry is the
// driver deferring work it was never able to attempt, such as a rate limit, and
// counting it would report an error rate for a queue that never errored.
const (
	retryJobSQL = `
		UPDATE convoy.queue_jobs
		SET status = $2,
		    run_at = CASE WHEN $3 <= NOW() + interval '1 second' THEN NOW() ELSE $3 END,
		    claimed_at = NULL,
		    last_error = $4,
		    updated_at = NOW()
		WHERE id = $1 AND status = $5`

	retryJobCountedSQL = `
		WITH moved AS (
			UPDATE convoy.queue_jobs
			SET status = $2,
			    retry_count = retry_count + 1,
			    run_at = CASE WHEN $3 <= NOW() + interval '1 second' THEN NOW() ELSE $3 END,
			    claimed_at = NULL,
			    last_error = $4,
			    updated_at = NOW()
			WHERE id = $1 AND status = $5
			RETURNING queue_name
		)
		` + recordFailureSQL

	// recordFailureSQL turns a preceding "retried"/"archived" CTE that returns
	// queue_name into one day-bucketed failure. It is appended rather than
	// repeated so both failure paths bucket the day the same way, and the
	// statement's RowsAffected stays 1 exactly when the update matched, which
	// is what the callers read to decide whether anything moved.
	recordFailureSQL = `
		INSERT INTO convoy.queue_job_stats AS s (queue_name, day, failed)
		SELECT queue_name, (NOW() AT TIME ZONE 'UTC')::date, 1 FROM moved
		ON CONFLICT (queue_name, day) DO UPDATE
		SET failed = s.failed + 1,
		    updated_at = NOW()`
)

func (q *PostgresQueue) Retry(ctx context.Context, id string, runAt time.Time, incrementRetry bool, lastError string) error {
	stmt := retryJobSQL
	if incrementRetry {
		stmt = retryJobCountedSQL
	}

	res, err := q.db.ExecContext(ctx, stmt, id, statusPending, runAt, lastError, statusProcessing)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n > 0 {
		q.notifyPending()
	}
	q.dropLeases([]string{id})
	return nil
}

func (q *PostgresQueue) Archive(ctx context.Context, id, lastError string) error {
	_, err := q.db.ExecContext(ctx, `
		WITH moved AS (
			UPDATE convoy.queue_jobs
			SET status = $2,
			    claimed_at = NULL,
			    last_error = $3,
			    updated_at = NOW()
			WHERE id = $1 AND status = $4
			RETURNING queue_name
		)
		`+recordFailureSQL,
		id, statusArchived, lastError, statusProcessing)
	if err != nil {
		return err
	}
	q.dropLeases([]string{id})
	return nil
}

func (q *PostgresQueue) Release(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	res, err := q.db.ExecContext(ctx, `
		UPDATE convoy.queue_jobs
		SET status = $2,
		    claimed_at = NULL,
		    updated_at = NOW()
		WHERE id = ANY($1) AND status = $3`,
		pq.Array(ids), statusPending, statusProcessing)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n > 0 {
		q.notifyPending()
	}
	q.dropLeases(ids)
	return nil
}

// Heartbeat renews the claim lease on jobs this consumer still owns. Matching
// only id and status = processing is not enough: ReclaimStuck returns the row
// to pending, the next Claim sets processing again, and a stale heartbeat
// would then extend the new owner's claimed_at. claim_id is minted at Claim
// and is not rewritten here, so aging claimed_at to simulate a slow handler
// still renews, and a later Claim's uuid does not.
func (q *PostgresQueue) Heartbeat(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	jobIDs, claimIDs := q.leasesForHeartbeat(ids)
	if len(jobIDs) == 0 {
		return nil
	}
	_, err := q.db.ExecContext(ctx, `
		UPDATE convoy.queue_jobs AS j
		SET claimed_at = NOW(),
		    updated_at = NOW()
		FROM UNNEST($1::text[], $2::uuid[]) AS t(id, claim_id)
		WHERE j.id = t.id
		  AND j.status = $3
		  AND j.claim_id = t.claim_id`,
		pq.Array(jobIDs), pq.Array(claimIDs), statusProcessing)
	return err
}

func (q *PostgresQueue) rememberClaims(jobs []queue.ClaimedJob, claimIDs []string) {
	q.leaseMu.Lock()
	defer q.leaseMu.Unlock()
	for i, job := range jobs {
		if claimIDs[i] == "" {
			continue
		}
		q.leases[job.ID] = claimIDs[i]
	}
}

func (q *PostgresQueue) dropLeases(ids []string) {
	q.leaseMu.Lock()
	defer q.leaseMu.Unlock()
	for _, id := range ids {
		delete(q.leases, id)
	}
}

func (q *PostgresQueue) leasesForHeartbeat(ids []string) ([]string, []string) {
	q.leaseMu.Lock()
	defer q.leaseMu.Unlock()
	jobIDs := make([]string, 0, len(ids))
	claimIDs := make([]string, 0, len(ids))
	for _, id := range ids {
		if cid, ok := q.leases[id]; ok {
			jobIDs = append(jobIDs, id)
			claimIDs = append(claimIDs, cid)
		}
	}
	return jobIDs, claimIDs
}

func (q *PostgresQueue) ReclaimStuck(ctx context.Context) (int64, error) {
	res, err := q.db.ExecContext(ctx, `
		UPDATE convoy.queue_jobs
		SET status = $1,
		    claimed_at = NULL,
		    claim_id = NULL,
		    updated_at = NOW()
		WHERE status = $2
		  AND claimed_at < NOW() - make_interval(secs => $3)`,
		statusPending, statusProcessing, q.stuckTimeout.Seconds())
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	if n > 0 {
		q.notifyPending()
	}
	return n, nil
}

// DeleteArchived drops finished rows. Cron tombstones are the memory that
// keeps one scheduler tick from being enqueued twice, so they are only dropped
// once they are older than any tick still in flight.
func (q *PostgresQueue) DeleteArchived(ctx context.Context) error {
	_, err := q.db.ExecContext(ctx, `
		DELETE FROM convoy.queue_jobs
		WHERE status = ANY($1)
		  AND (
			id NOT LIKE $2
			OR updated_at < NOW() - make_interval(secs => $3)
		  )`,
		pq.Array([]string{statusArchived, statusCompleted}),
		cronJobPrefix+"%",
		cronTombstoneRetention.Seconds())
	if err != nil {
		return err
	}

	// Throughput buckets past the window the monitor can chart are dead rows on
	// a table that gains one per queue per day forever. This runs on the same
	// nightly pass as the row cleanup because it is the same kind of debt.
	_, err = q.db.ExecContext(ctx, `
		DELETE FROM convoy.queue_job_stats
		WHERE day < (NOW() AT TIME ZONE 'UTC')::date - make_interval(days => $1::integer)`,
		queue.MaxHistoryDays)
	return err
}

// DeleteEventDeliveriesFromQueue drops pending and archived rows so a
// re-enqueue can insert a fresh pending job. Processing rows stay so the
// in-flight worker is not double-run; ReclaimStuck heals those.
func (q *PostgresQueue) DeleteEventDeliveriesFromQueue(queueName convoy.QueueName, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := q.db.ExecContext(context.Background(), `
		DELETE FROM convoy.queue_jobs
		WHERE queue_name = $1
		  AND id = ANY($2)
		  AND status <> $3`,
		string(queueName), pq.Array(ids), statusProcessing)
	return err
}

// QueueCount is per-queue depth for monitoring and Prometheus.
type QueueCount struct {
	QueueName  string `db:"queue_name" json:"queue"`
	Pending    int64  `db:"pending" json:"pending"`
	Processing int64  `db:"processing" json:"processing"`
	Archived   int64  `db:"archived" json:"archived"`
}

func (q *PostgresQueue) Counts(ctx context.Context) ([]QueueCount, error) {
	var rows []QueueCount
	err := q.db.SelectContext(ctx, &rows, `
		SELECT
			queue_name,
			COUNT(*) FILTER (WHERE status = 'pending') AS pending,
			COUNT(*) FILTER (WHERE status = 'processing') AS processing,
			COUNT(*) FILTER (WHERE status = 'archived') AS archived
		FROM convoy.queue_jobs
		WHERE status <> 'completed'
		GROUP BY queue_name
		ORDER BY queue_name`)
	if err != nil {
		return nil, err
	}
	return rows, nil
}
