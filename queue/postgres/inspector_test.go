package postgres

import (
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/queue"
)

func TestInspectorStats(t *testing.T) {
	q := setupQueue(t)
	ctx := t.Context()
	require.NoError(t, q.Write(ctx, convoy.EventProcessor, convoy.EventQueue, &queue.Job{
		ID:      ulid.Make().String(),
		Payload: []byte("x"),
	}))

	stats, err := q.Stats(ctx)
	require.NoError(t, err)
	require.Equal(t, queue.ProviderPostgres, stats.Provider)
	require.Equal(t, []string{queue.StatusPending, queue.StatusProcessing, queue.StatusArchived}, stats.Statuses)
	require.Len(t, stats.Queues, 1)
	require.Equal(t, string(convoy.EventQueue), stats.Queues[0].Queue)
	require.Equal(t, int64(1), stats.Queues[0].Counts[queue.StatusPending])
	require.Equal(t, int64(0), stats.Queues[0].Counts[queue.StatusArchived])
}

func TestInspectorTasks(t *testing.T) {
	q := setupQueue(t)
	ctx := t.Context()
	id := ulid.Make().String()
	require.NoError(t, q.Write(ctx, convoy.EventProcessor, convoy.EventQueue, &queue.Job{
		ID:      id,
		Payload: []byte("secret-payload"),
	}))

	page, err := q.Tasks(ctx, queue.TaskFilter{Queue: string(convoy.EventQueue), Status: queue.StatusPending, Page: 1})
	require.NoError(t, err)
	require.Len(t, page.Tasks, 1)
	require.Equal(t, id, page.Tasks[0].ID)
	require.Equal(t, string(convoy.EventProcessor), page.Tasks[0].TaskName)
	require.Equal(t, string(convoy.EventQueue), page.Tasks[0].Queue)
	require.Equal(t, queue.StatusPending, page.Tasks[0].Status)
	require.NotNil(t, page.Tasks[0].NextRunAt, "a pending row is waiting on run_at")
	require.Equal(t, 1, page.Page)
	require.False(t, page.HasNext)

	// A queue the caller did not name must not leak into the page.
	page, err = q.Tasks(ctx, queue.TaskFilter{Queue: string(convoy.RetryEventQueue), Status: queue.StatusPending, Page: 1})
	require.NoError(t, err)
	require.Empty(t, page.Tasks)

	page, err = q.Tasks(ctx, queue.TaskFilter{Queue: string(convoy.EventQueue), Status: queue.StatusArchived, Page: 1})
	require.NoError(t, err)
	require.Empty(t, page.Tasks)

	// A page past the end is empty rather than an error, so paging forward
	// after an archive drain does not fail.
	page, err = q.Tasks(ctx, queue.TaskFilter{Queue: string(convoy.EventQueue), Status: queue.StatusPending, Page: 2})
	require.NoError(t, err)
	require.Empty(t, page.Tasks)
	require.Equal(t, 2, page.Page)
}

// The page offers exactly what the update will take, so an operator is never
// shown a button the broker rejects.
func TestInspectorTaskActions(t *testing.T) {
	q := setupQueue(t)
	ctx := t.Context()
	name := string(convoy.EventQueue)

	tasks := func(status string) []queue.Task {
		page, err := q.Tasks(ctx, queue.TaskFilter{Queue: name, Status: status, Page: 1})
		require.NoError(t, err)
		return page.Tasks
	}

	id := ulid.Make().String()
	require.NoError(t, q.Write(ctx, convoy.EventProcessor, convoy.EventQueue, &queue.Job{
		ID:      id,
		Payload: []byte("x"),
	}))
	require.Equal(t, []string{queue.ActionArchive}, tasks(queue.StatusPending)[0].Actions)

	cronID := cronJobPrefix + ulid.Make().String()
	_, err := q.db.Exec(`
		INSERT INTO convoy.queue_jobs (id, task_name, queue_name, payload, status, run_at)
		VALUES ($1, $2, $3, $4, $5, NOW())`,
		cronID, string(convoy.EventProcessor), name, []byte("x"), statusPending)
	require.NoError(t, err)
	for _, task := range tasks(queue.StatusPending) {
		if task.ID == cronID {
			require.Empty(t, task.Actions, "scheduler rows are not an operator's to move")
		}
	}

	claimed, err := q.Claim(ctx, []string{name}, 10)
	require.NoError(t, err)
	require.NotEmpty(t, claimed)
	require.Empty(t, tasks(queue.StatusProcessing)[0].Actions, "a live claim owns the row")

	_, err = q.db.Exec(`UPDATE convoy.queue_jobs SET status = $2, claimed_at = NULL WHERE id = $1`, id, statusPending)
	require.NoError(t, err)
	require.NoError(t, q.ArchiveTask(ctx, name, id))
	archived := tasks(queue.StatusArchived)
	require.Len(t, archived, 1)
	require.Equal(t, []string{queue.ActionRetry}, archived[0].Actions)
	// run_at on an archived row is the run that already happened, and asynq
	// reports nothing there, so the column stays empty on both providers.
	require.Nil(t, archived[0].NextRunAt)
}

func TestInspectorTasksRejectsBadFilters(t *testing.T) {
	q := setupQueue(t)
	ctx := t.Context()

	_, err := q.Tasks(ctx, queue.TaskFilter{Status: queue.StatusPending, Page: 1})
	require.ErrorIs(t, err, queue.ErrQueueRequired)

	// completed rows are cron tombstones, and an unknown status must not be
	// answered with an empty page that reads as "nothing queued".
	for _, status := range []string{"completed", "bogus", "", queue.StatusRetry} {
		_, err = q.Tasks(ctx, queue.TaskFilter{Queue: string(convoy.EventQueue), Status: status, Page: 1})
		require.ErrorIs(t, err, queue.ErrUnknownTaskStatus, status)
	}

	// A page large enough to overflow the offset is rejected, not turned into
	// a negative offset the database refuses.
	for _, page := range []int{0, -1, queue.MaxTaskPage + 1} {
		_, err = q.Tasks(ctx, queue.TaskFilter{Queue: string(convoy.EventQueue), Status: queue.StatusPending, Page: page})
		require.ErrorIs(t, err, queue.ErrInvalidPage)
	}
}

func TestInspectorTasksPaginates(t *testing.T) {
	q := setupQueue(t)
	ctx := t.Context()
	for range queue.TasksPerPage + 1 {
		require.NoError(t, q.Write(ctx, convoy.EventProcessor, convoy.EventQueue, &queue.Job{
			ID:      ulid.Make().String(),
			Payload: []byte("x"),
		}))
	}

	first, err := q.Tasks(ctx, queue.TaskFilter{Queue: string(convoy.EventQueue), Status: queue.StatusPending, Page: 1})
	require.NoError(t, err)
	require.Len(t, first.Tasks, queue.TasksPerPage)
	require.True(t, first.HasNext)

	second, err := q.Tasks(ctx, queue.TaskFilter{Queue: string(convoy.EventQueue), Status: queue.StatusPending, Page: 2})
	require.NoError(t, err)
	require.Len(t, second.Tasks, 1)
	require.False(t, second.HasNext)

	// Rows written in the same statement share run_at, so without the id
	// tiebreaker the two pages could return the same row twice.
	seen := map[string]bool{}
	for _, task := range append(first.Tasks, second.Tasks...) {
		require.False(t, seen[task.ID], "task %s appeared on both pages", task.ID)
		seen[task.ID] = true
	}
	require.Len(t, seen, queue.TasksPerPage+1)
}

func taskStatus(t *testing.T, q *PostgresQueue, id string) string {
	t.Helper()
	var status string
	require.NoError(t, q.db.Get(&status, `SELECT status FROM convoy.queue_jobs WHERE id = $1`, id))
	return status
}

func TestArchiveAndRetryTask(t *testing.T) {
	q := setupQueue(t)
	ctx := t.Context()
	name := string(convoy.EventQueue)
	id := ulid.Make().String()
	require.NoError(t, q.Write(ctx, convoy.EventProcessor, convoy.EventQueue, &queue.Job{
		ID:      id,
		Payload: []byte("x"),
	}))

	require.NoError(t, q.ArchiveTask(ctx, name, id))
	require.Equal(t, statusArchived, taskStatus(t, q, id))

	// The row is here because it exhausted its budget, so the operator retry
	// has to clear the count or the first attempt archives it again.
	_, err := q.db.Exec(`UPDATE convoy.queue_jobs SET retry_count = max_retry WHERE id = $1`, id)
	require.NoError(t, err)

	require.ErrorIs(t, q.ArchiveTask(ctx, name, id), queue.ErrTaskStatusConflict)

	require.NoError(t, q.RetryTask(ctx, name, id))
	require.Equal(t, statusPending, taskStatus(t, q, id))

	var retryCount int
	require.NoError(t, q.db.Get(&retryCount, `SELECT retry_count FROM convoy.queue_jobs WHERE id = $1`, id))
	require.Zero(t, retryCount)

	require.ErrorIs(t, q.RetryTask(ctx, name, id), queue.ErrTaskStatusConflict)
	require.ErrorIs(t, q.RetryTask(ctx, name, "no-such-task"), queue.ErrTaskNotFound)
	require.ErrorIs(t, q.ArchiveTask(ctx, name, "no-such-task"), queue.ErrTaskNotFound)
}

// The queue name is part of the address, so an action aimed at another queue
// must not move a row the caller never saw.
func TestTaskActionsScopeToQueue(t *testing.T) {
	q := setupQueue(t)
	ctx := t.Context()
	id := ulid.Make().String()
	require.NoError(t, q.Write(ctx, convoy.EventProcessor, convoy.EventQueue, &queue.Job{
		ID:      id,
		Payload: []byte("x"),
	}))

	require.ErrorIs(t, q.ArchiveTask(ctx, string(convoy.RetryEventQueue), id), queue.ErrTaskNotFound)
	require.Equal(t, statusPending, taskStatus(t, q, id))
}

// A claimed row belongs to a live worker, which will complete or retry it.
// Archiving underneath that worker would let both happen.
func TestArchiveTaskRefusesProcessingRow(t *testing.T) {
	q := setupQueue(t)
	ctx := t.Context()
	id := ulid.Make().String()
	require.NoError(t, q.Write(ctx, convoy.EventProcessor, convoy.EventQueue, &queue.Job{
		ID:      id,
		Payload: []byte("x"),
	}))

	jobs, err := q.Claim(ctx, []string{string(convoy.EventQueue)}, 10)
	require.NoError(t, err)
	require.Len(t, jobs, 1)

	require.ErrorIs(t, q.ArchiveTask(ctx, string(convoy.EventQueue), id), queue.ErrTaskStatusConflict)
	require.Equal(t, statusProcessing, taskStatus(t, q, id))
}

// An archived cron row is the tombstone that keeps one tick from firing twice,
// so neither action may move it.
func TestTaskActionsRefuseSchedulerRows(t *testing.T) {
	q := setupQueue(t)
	ctx := t.Context()
	name := string(convoy.EventQueue)
	id := cronJobPrefix + ulid.Make().String()

	_, err := q.db.Exec(`
		INSERT INTO convoy.queue_jobs (id, task_name, queue_name, payload, status, run_at)
		VALUES ($1, $2, $3, $4, $5, NOW())`,
		id, string(convoy.EventProcessor), string(convoy.EventQueue), []byte("x"), statusArchived)
	require.NoError(t, err)

	require.ErrorIs(t, q.RetryTask(ctx, name, id), queue.ErrCronTaskImmutable)
	require.Equal(t, statusArchived, taskStatus(t, q, id))

	_, err = q.db.Exec(`UPDATE convoy.queue_jobs SET status = $2 WHERE id = $1`, id, statusPending)
	require.NoError(t, err)
	require.ErrorIs(t, q.ArchiveTask(ctx, name, id), queue.ErrCronTaskImmutable)
	require.Equal(t, statusPending, taskStatus(t, q, id))
}
