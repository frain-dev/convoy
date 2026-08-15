package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/internal/pkg/queue/tracectx"
	"github.com/frain-dev/convoy/queue"
)

var (
	_ queue.Queuer   = (*PostgresQueue)(nil)
	_ queue.Monitor  = (*PostgresQueue)(nil)
	_ queue.Archiver = (*PostgresQueue)(nil)

	// ErrTaskNotFound is returned by LastTaskError when no row matches.
	// Failure policy: fail closed — unknown is not "no prior error".
	ErrTaskNotFound = errors.New("postgres queue: task not found")
)

const (
	statusPending    = "pending"
	statusProcessing = "processing"
	statusArchived   = "archived"
	statusCompleted  = "completed"

	postgresBatchSize    = 64
	postgresBatchWait    = 2 * time.Millisecond
	postgresBatchTimeout = 30 * time.Second
	postgresWeightDepth  = 8

	// defaultMaxRetry matches asynq's unexported DefaultMaxRetry.
	defaultMaxRetry = 25

	// DefaultStuckTimeout is the claim lease. A live handler that outlives this
	// can be claimed again (double-run). Failure policy: fail open for crashed
	// workers so the queue does not stall; 30m matches asynq's default timeout.
	DefaultStuckTimeout = 30 * time.Minute

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
		last_error = NULL,
		updated_at = NOW()
	WHERE convoy.queue_jobs.status <> $9
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
		last_error = NULL,
		updated_at = NOW()
	WHERE convoy.queue_jobs.status <> $9
	  AND NOT (
		convoy.queue_jobs.id LIKE $10
		AND convoy.queue_jobs.status IN ($11, $12)
	  )
	RETURNING id`

// PostgresQueue implements queue.Queuer with convoy.queue_jobs as the broker.
type PostgresQueue struct {
	db           *sqlx.DB
	opts         queue.QueueOptions
	stuckTimeout time.Duration
	writes       chan writeRequest
	completions  chan completeRequest
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

func NewQueue(opts queue.QueueOptions) (*PostgresQueue, error) {
	if opts.DB == nil {
		return nil, errMissingDB
	}
	q := &PostgresQueue{
		db:           opts.DB,
		opts:         opts,
		stuckTimeout: DefaultStuckTimeout,
		writes:       make(chan writeRequest, postgresBatchSize),
		completions:  make(chan completeRequest, postgresBatchSize),
	}
	go q.runWriteBatcher()
	go q.runCompleteBatcher()
	return q, nil
}

func (q *PostgresQueue) Options() queue.QueueOptions {
	return q.opts
}

func (q *PostgresQueue) SetStuckTimeout(d time.Duration) {
	q.stuckTimeout = d
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
	select {
	case q.writes <- req:
	case <-ctx.Done():
		return ctx.Err()
	}

	// Failure policy: after admission, wait for the durable commit even if the
	// caller cancels. Returning early could let a parent job complete before
	// its child is durable; deterministic job IDs make an ambiguous retry safe.
	return <-req.result
}

func (q *PostgresQueue) runWriteBatcher() {
	for first := range q.writes {
		batch := []writeRequest{first}
		timer := time.NewTimer(postgresBatchWait)

	collect:
		for len(batch) < postgresBatchSize {
			select {
			case req := <-q.writes:
				batch = append(batch, req)
			case <-timer.C:
				break collect
			}
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
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
		results[0] = q.execWrite(ctx, q.db, batch[0])
		return results
	}

	// ON CONFLICT DO UPDATE cannot touch the same row twice in one command, so a
	// window holding the same job ID twice collapses to the last request. That
	// matches the sequential path, where the later write overwrote the earlier one.
	last := make(map[string]int, len(batch))
	for i := range batch {
		last[batch[i].id] = i
	}

	n := len(last)
	ids := make([]string, 0, n)
	taskNames := make([]string, 0, n)
	queueNames := make([]string, 0, n)
	payloads := make([][]byte, 0, n)
	headers := make([]string, 0, n)
	maxRetries := make([]int64, 0, n)
	delays := make([]float64, 0, n)
	for i := range batch {
		if last[batch[i].id] != i {
			continue
		}
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
	return results
}

// skippedWriteResult explains a row the conflict guard rejected. A cron tick
// that already fired is expected and idempotent; anything else is a job whose
// active claim must resolve before it can be re-enqueued.
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
		    updated_at = NOW()
		FROM claim
		WHERE j.id = claim.id
		RETURNING j.id, j.task_name, j.queue_name, j.payload, j.headers, j.max_retry, j.retry_count`,
		pq.Array(queueNames), limit, statusProcessing, statusPending, postgresWeightDepth,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []queue.ClaimedJob
	for rows.Next() {
		var (
			job         queue.ClaimedJob
			headerBytes []byte
		)
		if err := rows.Scan(&job.ID, &job.TaskName, &job.QueueName, &job.Payload, &headerBytes, &job.MaxRetry, &job.RetryCount); err != nil {
			return nil, err
		}
		if len(headerBytes) > 0 {
			if err := json.Unmarshal(headerBytes, &job.Headers); err != nil {
				return nil, fmt.Errorf("unmarshal job headers: %w", err)
			}
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (q *PostgresQueue) Complete(ctx context.Context, id string) error {
	req := completeRequest{id: id, result: make(chan error, 1)}
	select {
	case q.completions <- req:
	case <-ctx.Done():
		return ctx.Err()
	}
	return <-req.result
}

func (q *PostgresQueue) runCompleteBatcher() {
	for first := range q.completions {
		batch := []completeRequest{first}
		timer := time.NewTimer(postgresBatchWait)

	collect:
		for len(batch) < postgresBatchSize {
			select {
			case req := <-q.completions:
				batch = append(batch, req)
			case <-timer.C:
				break collect
			}
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
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

	_, err := q.db.ExecContext(ctx, `
		WITH completed_cron AS (
			UPDATE convoy.queue_jobs
			SET status = $3,
			    claimed_at = NULL,
			    updated_at = NOW()
			WHERE id = ANY($1)
			  AND status = $2
			  AND id LIKE $4
		)
		DELETE FROM convoy.queue_jobs
		WHERE id = ANY($1)
		  AND status = $2
		  AND id NOT LIKE $4`,
		pq.Array(ids), statusProcessing, statusCompleted, cronJobPrefix+"%",
	)
	return err
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

func (q *PostgresQueue) Retry(ctx context.Context, id string, runAt time.Time, incrementRetry bool, lastError string) error {
	// Snap "now"/past Go times onto PostgreSQL NOW() so Claim's run_at <= NOW()
	// cannot miss an immediate retry when the two clocks disagree by milliseconds.
	if incrementRetry {
		_, err := q.db.ExecContext(ctx, `
			UPDATE convoy.queue_jobs
			SET status = $2,
			    retry_count = retry_count + 1,
			    run_at = CASE WHEN $3 <= NOW() + interval '1 second' THEN NOW() ELSE $3 END,
			    claimed_at = NULL,
			    last_error = $4,
			    updated_at = NOW()
			WHERE id = $1 AND status = $5`,
			id, statusPending, runAt, lastError, statusProcessing)
		return err
	}
	_, err := q.db.ExecContext(ctx, `
		UPDATE convoy.queue_jobs
		SET status = $2,
		    run_at = CASE WHEN $3 <= NOW() + interval '1 second' THEN NOW() ELSE $3 END,
		    claimed_at = NULL,
		    last_error = $4,
		    updated_at = NOW()
		WHERE id = $1 AND status = $5`,
		id, statusPending, runAt, lastError, statusProcessing)
	return err
}

func (q *PostgresQueue) Archive(ctx context.Context, id, lastError string) error {
	_, err := q.db.ExecContext(ctx, `
		UPDATE convoy.queue_jobs
		SET status = $2,
		    claimed_at = NULL,
		    last_error = $3,
		    updated_at = NOW()
		WHERE id = $1 AND status = $4`,
		id, statusArchived, lastError, statusProcessing)
	return err
}

func (q *PostgresQueue) Release(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := q.db.ExecContext(ctx, `
		UPDATE convoy.queue_jobs
		SET status = $2,
		    claimed_at = NULL,
		    updated_at = NOW()
		WHERE id = ANY($1) AND status = $3`,
		pq.Array(ids), statusPending, statusProcessing)
	return err
}

func (q *PostgresQueue) ReclaimStuck(ctx context.Context) (int64, error) {
	res, err := q.db.ExecContext(ctx, `
		UPDATE convoy.queue_jobs
		SET status = $1,
		    claimed_at = NULL,
		    updated_at = NOW()
		WHERE status = $2
		  AND claimed_at < NOW() - make_interval(secs => $3)`,
		statusPending, statusProcessing, q.stuckTimeout.Seconds())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
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

func (q *PostgresQueue) QueueNames() []string {
	names := make([]string, 0, len(q.opts.Names))
	for name := range q.opts.Names {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// QueuePriorityCycle returns a smooth weighted round-robin cycle matching the
// weights passed to asynq in Redis mode.
func (q *PostgresQueue) QueuePriorityCycle() []string {
	return queue.PriorityCycle(q.opts.Names)
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
