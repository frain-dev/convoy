package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

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

// actions mirrors what RetryTask and ArchiveTask below will accept, so the page
// only offers what the update will take. A processing row is held by a live
// worker that would finish or retry the job underneath an archive, and a cron
// row is the tombstone that stops one tick being enqueued twice, so neither is
// an operator's to move.
func (r taskRow) actions() []string {
	if strings.HasPrefix(r.ID, cronJobPrefix) {
		return []string{}
	}
	switch r.Status {
	case statusPending:
		return []string{queue.ActionArchive}
	case statusArchived:
		return []string{queue.ActionRetry}
	default:
		return []string{}
	}
}

func (q *PostgresQueue) Stats(ctx context.Context) (queue.Stats, error) {
	counts, err := q.Counts(ctx)
	if err != nil {
		return queue.Stats{}, err
	}

	stats := queue.Stats{
		Provider: queue.ProviderPostgres,
		Statuses: inspectorStatuses,
		Queues:   make([]queue.QueueStat, 0, len(counts)),
	}
	for _, c := range counts {
		stats.Queues = append(stats.Queues, queue.QueueStat{
			Queue: c.QueueName,
			Counts: map[string]int64{
				queue.StatusPending:    c.Pending,
				queue.StatusProcessing: c.Processing,
				queue.StatusArchived:   c.Archived,
			},
		})
	}
	return stats, nil
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
	limit := queue.TasksPerPage + 1
	offset := (f.Page - 1) * queue.TasksPerPage

	rows := []taskRow{}
	err := q.db.SelectContext(ctx, &rows, `
		SELECT id, task_name, queue_name, status, retry_count, max_retry,
		       run_at, claimed_at, last_error, created_at
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
	if len(rows) > queue.TasksPerPage {
		rows = rows[:queue.TasksPerPage]
		page.HasNext = true
	}
	page.Tasks = make([]queue.Task, 0, len(rows))
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

// taskTransitionError explains a zero-row transition. A lookup failure is
// returned as itself rather than as "not found", so a caller never reports a
// row as absent on the strength of a failed read. The lookup is scoped by queue
// too: a row that exists under another queue name is not the row the caller
// asked about.
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
