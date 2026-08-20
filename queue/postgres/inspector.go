package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"

	"github.com/frain-dev/convoy/queue"
)

// inspectorStatuses is the vocabulary this provider serves, in the order the
// dashboard shows them. There is no scheduled or retry set: a delivery waiting
// on a backoff is a pending row with a future run_at, and completed rows are
// cron tombstones that no operator should act on.
var inspectorStatuses = []string{queue.StatusPending, queue.StatusProcessing, queue.StatusArchived}

// taskRow scans one drill-down row. It stays private to this package so the
// shared queue.Task carries no database tags and no provider-specific columns.
type taskRow struct {
	ID         string     `db:"id"`
	TaskName   string     `db:"task_name"`
	QueueName  string     `db:"queue_name"`
	Status     string     `db:"status"`
	RetryCount int        `db:"retry_count"`
	MaxRetry   int        `db:"max_retry"`
	RunAt      time.Time  `db:"run_at"`
	ClaimedAt  *time.Time `db:"claimed_at"`
	LastError  *string    `db:"last_error"`
	CreatedAt  time.Time  `db:"created_at"`
}

const taskColumns = `id, task_name, queue_name, status, retry_count, max_retry,
		       run_at, claimed_at, last_error, created_at`

func (r taskRow) task() queue.Task {
	t := queue.Task{
		ID:         r.ID,
		Queue:      r.QueueName,
		TaskName:   r.TaskName,
		Status:     r.Status,
		RetryCount: r.RetryCount,
		MaxRetry:   r.MaxRetry,
		ClaimedAt:  r.ClaimedAt,
		CreatedAt:  &r.CreatedAt,
		Actions:    r.actions(),
	}
	// run_at is only a next run while the row is still waiting to be claimed.
	// On a processing or archived row it is the time of the run that already
	// happened, and asynq reports nothing there, so publishing it would make
	// the same column mean different things on the two providers.
	if r.Status == statusPending {
		t.NextRunAt = &r.RunAt
	}
	if r.LastError != nil {
		t.LastError = *r.LastError
	}
	return t
}

// actions mirrors what the mutations below will accept, so the page only offers
// what the update will take. A processing row is held by a live worker that
// would finish or retry the job underneath an archive, and a cron row is the
// tombstone that stops one tick being enqueued twice, so neither is an
// operator's to move.
func (r taskRow) actions() []string {
	if strings.HasPrefix(r.ID, cronJobPrefix) {
		return []string{}
	}
	switch r.Status {
	case statusPending:
		// A pending row that is already due is being claimed as fast as the
		// workers can take it, so pulling it forward is not an action; one
		// waiting on a backoff can be run now.
		if r.RunAt.After(time.Now()) {
			return []string{queue.ActionRun, queue.ActionArchive, queue.ActionDelete}
		}
		return []string{queue.ActionArchive, queue.ActionDelete}
	case statusArchived:
		return []string{queue.ActionRetry, queue.ActionDelete}
	default:
		return []string{}
	}
}

// queueStatRow is one queue's landing-page line. The counts, the pause flag,
// the latency and today's throughput come from four different tables' worth of
// state, so they are assembled in one query rather than one call per queue.
type queueStatRow struct {
	QueueName  string  `db:"queue_name"`
	Pending    int64   `db:"pending"`
	Processing int64   `db:"processing"`
	Archived   int64   `db:"archived"`
	Paused     bool    `db:"paused"`
	LatencyMS  float64 `db:"latency_ms"`
	Processed  int64   `db:"processed"`
	Failed     int64   `db:"failed"`
}

func (q *PostgresQueue) Stats(ctx context.Context) (queue.Stats, error) {
	rows := []queueStatRow{}
	// Queues come from the union of what holds rows, what an operator has
	// paused and what ran today: a queue that is paused and fully drained, or
	// one that finished everything an hour ago, still has to be visible, and
	// only the last of those is answered by counting queue_jobs.
	err := q.db.SelectContext(ctx, &rows, `
		WITH names AS (
			SELECT queue_name FROM convoy.queue_jobs
			UNION
			SELECT queue_name FROM convoy.queue_state WHERE paused_at IS NOT NULL
			UNION
			SELECT queue_name FROM convoy.queue_job_stats
			WHERE day = (NOW() AT TIME ZONE 'UTC')::date
		),
		counts AS (
			SELECT queue_name,
			       COUNT(*) FILTER (WHERE status = $1) AS pending,
			       COUNT(*) FILTER (WHERE status = $2) AS processing,
			       COUNT(*) FILTER (WHERE status = $3) AS archived,
			       -- How far behind the workers are: the age of the oldest task
			       -- that is due. A task scheduled for later is not late.
			       COALESCE(EXTRACT(EPOCH FROM
			           NOW() - MIN(run_at) FILTER (WHERE status = $1 AND run_at <= NOW())
			       ) * 1000, 0) AS latency_ms
			FROM convoy.queue_jobs
			GROUP BY queue_name
		)
		SELECT n.queue_name,
		       COALESCE(c.pending, 0) AS pending,
		       COALESCE(c.processing, 0) AS processing,
		       COALESCE(c.archived, 0) AS archived,
		       COALESCE(c.latency_ms, 0) AS latency_ms,
		       (s.paused_at IS NOT NULL) AS paused,
		       COALESCE(t.processed, 0) AS processed,
		       COALESCE(t.failed, 0) AS failed
		FROM names n
		LEFT JOIN counts c ON c.queue_name = n.queue_name
		LEFT JOIN convoy.queue_state s ON s.queue_name = n.queue_name
		LEFT JOIN convoy.queue_job_stats t
		       ON t.queue_name = n.queue_name
		      AND t.day = (NOW() AT TIME ZONE 'UTC')::date
		ORDER BY n.queue_name`,
		statusPending, statusProcessing, statusArchived)
	if err != nil {
		return queue.Stats{}, err
	}

	stats := queue.Stats{
		Provider: queue.ProviderPostgres,
		Statuses: inspectorStatuses,
		Queues:   make([]queue.QueueStat, 0, len(rows)),
	}
	for _, r := range rows {
		stats.Queues = append(stats.Queues, queue.QueueStat{
			Queue: r.QueueName,
			Counts: map[string]int64{
				queue.StatusPending:    r.Pending,
				queue.StatusProcessing: r.Processing,
				queue.StatusArchived:   r.Archived,
			},
			Paused:    r.Paused,
			LatencyMS: int64(r.LatencyMS),
			Processed: r.Processed,
			Failed:    r.Failed,
		})
	}
	return stats, nil
}

// History returns the daily throughput series. Days with no work are filled in
// from a generated calendar rather than left out, so the chart plots a real gap
// instead of drawing two distant days as adjacent.
func (q *PostgresQueue) History(ctx context.Context, queueName string, days int) ([]queue.HistoryPoint, error) {
	if queueName == "" {
		return nil, queue.ErrQueueRequired
	}
	days = historyWindow(days)

	points := []queue.HistoryPoint{}
	err := q.db.SelectContext(ctx, &points, `
		SELECT TO_CHAR(d.day, 'YYYY-MM-DD') AS date,
		       COALESCE(s.processed, 0) AS processed,
		       COALESCE(s.failed, 0) AS failed
		FROM generate_series(
			(NOW() AT TIME ZONE 'UTC')::date - make_interval(days => $2::integer - 1),
			(NOW() AT TIME ZONE 'UTC')::date,
			interval '1 day'
		) AS d(day)
		LEFT JOIN convoy.queue_job_stats s
		       ON s.day = d.day::date
		      AND s.queue_name = $1
		ORDER BY d.day`,
		queueName, days)
	if err != nil {
		return nil, err
	}
	return points, nil
}

func historyWindow(days int) int {
	if days < 1 {
		return queue.DefaultHistoryDays
	}
	if days > queue.MaxHistoryDays {
		return queue.MaxHistoryDays
	}
	return days
}

// SchedulerEntries reads what the scheduler published. The registry is written
// by the process running the scheduler, which is not the process serving this
// request, so an instance with no agent running returns an empty list rather
// than an error: nothing registered is a true answer.
func (q *PostgresQueue) SchedulerEntries(ctx context.Context) ([]queue.SchedulerEntry, error) {
	rows := []schedulerEntryRow{}
	err := q.db.SelectContext(ctx, &rows, `
		SELECT id, task_name, queue_name, spec, next_run_at, prev_run_at
		FROM convoy.queue_scheduler_entries
		ORDER BY task_name, id`)
	if err != nil {
		return nil, err
	}

	entries := make([]queue.SchedulerEntry, 0, len(rows))
	for _, r := range rows {
		entries = append(entries, queue.SchedulerEntry{
			ID:       r.ID,
			Spec:     r.Spec,
			TaskName: r.TaskName,
			Queue:    r.QueueName,
			NextRun:  r.NextRunAt,
			PrevRun:  r.PrevRunAt,
		})
	}
	return entries, nil
}

// RecordSchedulerEntry publishes one registered periodic task. prev_run_at is
// only overwritten when the caller has one: a replica restarting re-registers
// every entry, and taking its empty previous run would erase the last firing
// another replica recorded.
func (q *PostgresQueue) RecordSchedulerEntry(ctx context.Context, e queue.SchedulerEntry) error {
	_, err := q.db.ExecContext(ctx, `
		INSERT INTO convoy.queue_scheduler_entries AS s
			(id, task_name, queue_name, spec, next_run_at, prev_run_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
		ON CONFLICT (id) DO UPDATE
		SET task_name = EXCLUDED.task_name,
		    queue_name = EXCLUDED.queue_name,
		    spec = EXCLUDED.spec,
		    next_run_at = EXCLUDED.next_run_at,
		    prev_run_at = COALESCE(EXCLUDED.prev_run_at, s.prev_run_at),
		    updated_at = NOW()`,
		e.ID, e.TaskName, e.Queue, e.Spec, e.NextRun, e.PrevRun)
	return err
}

func (q *PostgresQueue) PruneSchedulerEntries(ctx context.Context, keepIDs []string) error {
	if len(keepIDs) == 0 {
		return nil
	}
	_, err := q.db.ExecContext(ctx, `
		DELETE FROM convoy.queue_scheduler_entries WHERE NOT (id = ANY($1))`,
		pq.Array(keepIDs))
	return err
}

type schedulerEntryRow struct {
	ID        string     `db:"id"`
	TaskName  string     `db:"task_name"`
	QueueName string     `db:"queue_name"`
	Spec      string     `db:"spec"`
	NextRunAt *time.Time `db:"next_run_at"`
	PrevRunAt *time.Time `db:"prev_run_at"`
}

// Tasks lists one page of queued rows. Each status is ordered by the column an
// operator is actually looking down: pending in claim order, processing by
// longest-held claim, archived by most recent failure. id breaks ties so offset
// paging cannot repeat or skip a row when timestamps collide.
func (q *PostgresQueue) Tasks(ctx context.Context, f queue.TaskFilter) (queue.TaskPage, error) {
	if f.Queue == "" {
		return queue.TaskPage{}, queue.ErrQueueRequired
	}
	if f.Page < 1 || f.Page > queue.MaxTaskPage {
		return queue.TaskPage{}, fmt.Errorf("%w: page must be between 1 and %d", queue.ErrInvalidPage, queue.MaxTaskPage)
	}
	if f.Search != "" {
		return q.searchTask(ctx, f)
	}

	var order string
	switch f.Status {
	case statusPending:
		order = "run_at, id"
	case statusProcessing:
		order = "claimed_at, id"
	case statusArchived:
		order = "updated_at DESC, id"
	default:
		return queue.TaskPage{}, fmt.Errorf("%w: %q", queue.ErrUnknownTaskStatus, f.Status)
	}

	// One extra row answers "is there a next page" without a COUNT over a set
	// that grows with archive retention.
	size := f.Size()
	limit := size + 1
	offset := (f.Page - 1) * size

	rows := []taskRow{}
	err := q.db.SelectContext(ctx, &rows, `
		SELECT `+taskColumns+`
		FROM convoy.queue_jobs
		WHERE status = $1
		  AND queue_name = $2
		ORDER BY `+order+`
		LIMIT $3 OFFSET $4`,
		f.Status, f.Queue, limit, offset)
	if err != nil {
		return queue.TaskPage{}, err
	}

	page := queue.TaskPage{Page: f.Page}
	if len(rows) > size {
		rows = rows[:size]
		page.HasNext = true
	}
	page.Tasks = make([]queue.Task, 0, len(rows))
	for _, r := range rows {
		page.Tasks = append(page.Tasks, r.task())
	}
	return page, nil
}

// searchTask answers an exact id lookup. The status filter is deliberately not
// applied: an operator searching an id wants to know where that task is, and
// answering "not found" because it moved to another status would be a lie about
// the queue. A cron tombstone is excluded because it is not a task an operator
// can act on, and finding one would offer buttons that all refuse.
func (q *PostgresQueue) searchTask(ctx context.Context, f queue.TaskFilter) (queue.TaskPage, error) {
	page := queue.TaskPage{Page: 1, Tasks: []queue.Task{}}
	if f.Page > 1 {
		return page, nil
	}

	rows := []taskRow{}
	err := q.db.SelectContext(ctx, &rows, `
		SELECT `+taskColumns+`
		FROM convoy.queue_jobs
		WHERE id = $1 AND queue_name = $2 AND id NOT LIKE $3
		LIMIT 1`,
		f.Search, f.Queue, cronJobPrefix+"%")
	if err != nil {
		return queue.TaskPage{}, err
	}
	for _, r := range rows {
		page.Tasks = append(page.Tasks, r.task())
	}
	return page, nil
}

// RetryTask returns an archived row to the queue at an operator's request.
// retry_count resets because the row is here precisely because it exhausted
// max_retry, and leaving the count would archive it again on the first attempt.
// last_error is kept so the page still shows why it failed.
func (q *PostgresQueue) RetryTask(ctx context.Context, queueName, id string) error {
	res, err := q.db.ExecContext(ctx, `
		UPDATE convoy.queue_jobs
		SET status = $3,
		    retry_count = 0,
		    run_at = NOW(),
		    claimed_at = NULL,
		    updated_at = NOW()
		WHERE id = $1 AND queue_name = $2 AND status = $4 AND id NOT LIKE $5`,
		id, queueName, statusPending, statusArchived, cronJobPrefix+"%")
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return q.taskTransitionError(ctx, queueName, id, statusArchived)
	}
	q.notifyPending()
	return nil
}

// RunTask pulls a task waiting on a backoff forward to now. retry_count is left
// alone: the attempts already made still happened, and clearing them would give
// a task that has nearly exhausted its budget a fresh one every time an
// operator grew impatient.
func (q *PostgresQueue) RunTask(ctx context.Context, queueName, id string) error {
	res, err := q.db.ExecContext(ctx, `
		UPDATE convoy.queue_jobs
		SET run_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1 AND queue_name = $2 AND status = $3
		  AND run_at > NOW() AND id NOT LIKE $4`,
		id, queueName, statusPending, cronJobPrefix+"%")
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return q.taskTransitionError(ctx, queueName, id, "pending and waiting on a backoff")
	}
	q.notifyPending()
	return nil
}

// ArchiveTask takes a pending row out of the queue. Processing rows are not
// archivable: a live worker holds that claim and would complete or retry the
// job underneath the archive. ReclaimStuck returns an abandoned claim to
// pending first, and it can be archived then.
func (q *PostgresQueue) ArchiveTask(ctx context.Context, queueName, id string) error {
	res, err := q.db.ExecContext(ctx, `
		UPDATE convoy.queue_jobs
		SET status = $3,
		    claimed_at = NULL,
		    updated_at = NOW()
		WHERE id = $1 AND queue_name = $2 AND status = $4 AND id NOT LIKE $5`,
		id, queueName, statusArchived, statusPending, cronJobPrefix+"%")
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return q.taskTransitionError(ctx, queueName, id, statusPending)
	}
	return nil
}

// DeleteTask drops a row for good. Processing rows are excluded for the same
// reason archive excludes them, and this is the one action with nothing behind
// it: an archived row that is deleted cannot be retried later.
func (q *PostgresQueue) DeleteTask(ctx context.Context, queueName, id string) error {
	res, err := q.db.ExecContext(ctx, `
		DELETE FROM convoy.queue_jobs
		WHERE id = $1 AND queue_name = $2 AND status = ANY($3) AND id NOT LIKE $4`,
		id, queueName, pq.Array([]string{statusPending, statusArchived}), cronJobPrefix+"%")
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return q.taskTransitionError(ctx, queueName, id, statusPending+" or "+statusArchived)
	}
	return nil
}

// BulkAction runs one action over selected ids. Each id goes through the same
// single-task path rather than one wide statement, so a selection that has gone
// stale reports which rows moved and why the rest did not, instead of a count
// that leaves the operator guessing which of their rows it covered.
func (q *PostgresQueue) BulkAction(ctx context.Context, queueName, action string, ids []string) (queue.BulkResult, error) {
	return queue.RunBulkAction(ctx, q, queueName, action, ids)
}

// PauseQueue stops workers claiming from this queue. Claims already held run to
// completion; the claim query simply stops selecting new rows.
func (q *PostgresQueue) PauseQueue(ctx context.Context, queueName string) error {
	return q.setPaused(ctx, queueName, true)
}

// UnpauseQueue lets workers claim again. The pending rows waited where they
// were, so nothing has to be re-enqueued; the listener is nudged because the
// workers may be parked waiting for a notification that never came while the
// queue was paused.
func (q *PostgresQueue) UnpauseQueue(ctx context.Context, queueName string) error {
	if err := q.setPaused(ctx, queueName, false); err != nil {
		return err
	}
	q.notifyPending()
	return nil
}

func (q *PostgresQueue) setPaused(ctx context.Context, queueName string, paused bool) error {
	if queueName == "" {
		return queue.ErrQueueRequired
	}

	var pausedAt any
	if paused {
		pausedAt = time.Now()
	}
	_, err := q.db.ExecContext(ctx, `
		INSERT INTO convoy.queue_state (queue_name, paused_at, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (queue_name) DO UPDATE
		SET paused_at = EXCLUDED.paused_at,
		    updated_at = NOW()`,
		queueName, pausedAt)
	return err
}

// taskTransitionError explains a zero-row transition. A lookup failure is
// returned as itself rather than as "not found", so a caller never reports a
// row as absent on the strength of a failed read. The lookup is scoped by queue
// too: a row that exists under another queue name is not the row the caller
// asked about.
//
// want is the caller's whole requirement in words, not one status, because two
// of these actions accept more than one status and one of them also requires
// the row to be waiting on a backoff. Naming a single status there would tell
// an operator their pending task was refused for not being pending.
func (q *PostgresQueue) taskTransitionError(ctx context.Context, queueName, id, want string) error {
	var status string
	err := q.db.GetContext(ctx, &status, `
		SELECT status FROM convoy.queue_jobs WHERE id = $1 AND queue_name = $2`, id, queueName)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return queue.ErrTaskNotFound
	case err != nil:
		return err
	}
	if strings.HasPrefix(id, cronJobPrefix) {
		return queue.ErrCronTaskImmutable
	}
	return fmt.Errorf("%w: task is %s, want %s", queue.ErrTaskStatusConflict, status, want)
}
