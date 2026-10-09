package postgres

import (
	"testing"
	"time"

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
	// A pending row that is already due is being claimed as fast as the workers
	// can take it, so there is nothing to pull forward.
	require.Equal(t, []string{queue.ActionArchive, queue.ActionDelete}, tasks(queue.StatusPending)[0].Actions)

	// One waiting on a backoff can be run now.
	_, err := q.db.Exec(`UPDATE convoy.queue_jobs SET run_at = NOW() + interval '1 hour' WHERE id = $1`, id)
	require.NoError(t, err)
	require.Equal(t, []string{queue.ActionRun, queue.ActionArchive, queue.ActionDelete}, tasks(queue.StatusPending)[0].Actions)
	_, err = q.db.Exec(`UPDATE convoy.queue_jobs SET run_at = NOW() WHERE id = $1`, id)
	require.NoError(t, err)

	cronID := cronJobPrefix + ulid.Make().String()
	_, err = q.db.Exec(`
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
	require.Equal(t, []string{queue.ActionRetry, queue.ActionDelete}, archived[0].Actions)
	// run_at on an archived row is the run that already happened, and asynq
	// reports nothing there, so the column stays empty on both providers.
	require.Nil(t, archived[0].NextRunAt)
}

// Latency is how far behind the workers are, so a task scheduled for later is
// not late, and today's throughput comes from the counters the driver writes as
// tasks finish rather than from rows that no longer exist.
func TestInspectorStatsLatencyAndThroughput(t *testing.T) {
	q := setupQueue(t)
	ctx := t.Context()
	name := string(convoy.EventQueue)

	stat := func() queue.QueueStat {
		stats, err := q.Stats(ctx)
		require.NoError(t, err)
		for _, s := range stats.Queues {
			if s.Queue == name {
				return s
			}
		}
		t.Fatalf("queue %s not reported", name)
		return queue.QueueStat{}
	}

	future := ulid.Make().String()
	require.NoError(t, q.Write(ctx, convoy.EventProcessor, convoy.EventQueue, &queue.Job{ID: future, Payload: []byte("x"), Delay: time.Hour}))
	require.Zero(t, stat().LatencyMS, "a task scheduled for later is not late")

	due := ulid.Make().String()
	require.NoError(t, q.Write(ctx, convoy.EventProcessor, convoy.EventQueue, &queue.Job{ID: due, Payload: []byte("x")}))
	_, err := q.db.Exec(`UPDATE convoy.queue_jobs SET run_at = NOW() - interval '90 seconds' WHERE id = $1`, due)
	require.NoError(t, err)
	require.Greater(t, stat().LatencyMS, int64(60_000), "the oldest due task is over a minute old")

	// A completed task's row is deleted, so the counter is the only record it
	// ever ran.
	_, err = q.Claim(ctx, []string{name}, 10)
	require.NoError(t, err)
	require.NoError(t, q.Complete(ctx, due))
	require.Equal(t, int64(1), stat().Processed)

	// Archiving is a failure; an uncounted retry is not, because the driver
	// never got to attempt the work.
	failing := ulid.Make().String()
	require.NoError(t, q.Write(ctx, convoy.EventProcessor, convoy.EventQueue, &queue.Job{ID: failing, Payload: []byte("x")}))
	_, err = q.Claim(ctx, []string{name}, 10)
	require.NoError(t, err)
	require.NoError(t, q.Retry(ctx, failing, time.Now(), false, "rate limited"))
	require.Zero(t, stat().Failed, "a deferral is not an error")

	_, err = q.Claim(ctx, []string{name}, 10)
	require.NoError(t, err)
	require.NoError(t, q.Retry(ctx, failing, time.Now(), true, "boom"))
	require.Equal(t, int64(1), stat().Failed)

	_, err = q.Claim(ctx, []string{name}, 10)
	require.NoError(t, err)
	require.NoError(t, q.Archive(ctx, failing, "boom"))
	require.Equal(t, int64(2), stat().Failed, "failures count attempts, not tasks")
}

// History fills every day in the window so a chart plots a real gap instead of
// drawing two distant days as adjacent.
func TestInspectorHistory(t *testing.T) {
	q := setupQueue(t)
	ctx := t.Context()
	name := string(convoy.EventQueue)

	_, err := q.db.Exec(`
		INSERT INTO convoy.queue_job_stats (queue_name, day, processed, failed)
		VALUES ($1, (NOW() AT TIME ZONE 'UTC')::date, 12, 3)`, name)
	require.NoError(t, err)

	points, err := q.History(ctx, name, 7)
	require.NoError(t, err)
	require.Len(t, points, 7)
	require.Equal(t, int64(12), points[6].Processed)
	require.Equal(t, int64(3), points[6].Failed)
	require.Zero(t, points[0].Processed, "a day with no throughput is a zero, not a missing point")

	// The window is bounded on both ends rather than trusted.
	points, err = q.History(ctx, name, queue.MaxHistoryDays+50)
	require.NoError(t, err)
	require.Len(t, points, queue.MaxHistoryDays)

	_, err = q.History(ctx, "", 7)
	require.ErrorIs(t, err, queue.ErrQueueRequired)
}

// A paused queue keeps its work; the workers just stop taking it.
func TestPauseStopsClaim(t *testing.T) {
	q := setupQueue(t)
	ctx := t.Context()
	name := string(convoy.EventQueue)
	require.NoError(t, q.Write(ctx, convoy.EventProcessor, convoy.EventQueue, &queue.Job{
		ID:      ulid.Make().String(),
		Payload: []byte("x"),
	}))

	require.NoError(t, q.PauseQueue(ctx, name))
	claimed, err := q.Claim(ctx, []string{name}, 10)
	require.NoError(t, err)
	require.Empty(t, claimed, "a paused queue is not claimed from")

	stats, err := q.Stats(ctx)
	require.NoError(t, err)
	require.True(t, stats.Queues[0].Paused)
	require.Equal(t, int64(1), stats.Queues[0].Counts[queue.StatusPending], "the work is still there")

	require.NoError(t, q.UnpauseQueue(ctx, name))
	claimed, err = q.Claim(ctx, []string{name}, 10)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
}

// A paused queue with nothing left in it still has to be visible, or the
// operator who paused it has no way back.
func TestStatsReportsDrainedPausedQueue(t *testing.T) {
	q := setupQueue(t)
	ctx := t.Context()
	name := string(convoy.RetryEventQueue)

	require.NoError(t, q.PauseQueue(ctx, name))
	stats, err := q.Stats(ctx)
	require.NoError(t, err)
	require.Len(t, stats.Queues, 1)
	require.Equal(t, name, stats.Queues[0].Queue)
	require.True(t, stats.Queues[0].Paused)
}

// An operator searching an id wants to know where that task is. Answering "not
// found" because it moved to another status would be a lie about the queue.
func TestInspectorSearchIgnoresStatus(t *testing.T) {
	q := setupQueue(t)
	ctx := t.Context()
	name := string(convoy.EventQueue)
	id := ulid.Make().String()
	require.NoError(t, q.Write(ctx, convoy.EventProcessor, convoy.EventQueue, &queue.Job{ID: id, Payload: []byte("x")}))
	require.NoError(t, q.ArchiveTask(ctx, name, id))

	page, err := q.Tasks(ctx, queue.TaskFilter{Queue: name, Status: queue.StatusPending, Page: 1, Search: id})
	require.NoError(t, err)
	require.Len(t, page.Tasks, 1)
	require.Equal(t, queue.StatusArchived, page.Tasks[0].Status)
	require.False(t, page.HasNext)

	page, err = q.Tasks(ctx, queue.TaskFilter{Queue: name, Status: queue.StatusPending, Page: 1, Search: "no-such-task"})
	require.NoError(t, err)
	require.Empty(t, page.Tasks)

	// A cron tombstone is not a task an operator can act on, so finding one
	// would only offer buttons that all refuse.
	cronID := cronJobPrefix + ulid.Make().String()
	_, err = q.db.Exec(`
		INSERT INTO convoy.queue_jobs (id, task_name, queue_name, payload, status, run_at)
		VALUES ($1, $2, $3, $4, $5, NOW())`,
		cronID, string(convoy.EventProcessor), name, []byte("x"), statusPending)
	require.NoError(t, err)
	page, err = q.Tasks(ctx, queue.TaskFilter{Queue: name, Status: queue.StatusPending, Page: 1, Search: cronID})
	require.NoError(t, err)
	require.Empty(t, page.Tasks)
}

func TestRunAndDeleteTask(t *testing.T) {
	q := setupQueue(t)
	ctx := t.Context()
	name := string(convoy.EventQueue)
	id := ulid.Make().String()
	require.NoError(t, q.Write(ctx, convoy.EventProcessor, convoy.EventQueue, &queue.Job{ID: id, Payload: []byte("x"), Delay: time.Hour}))

	// The attempts already made still happened: pulling a task forward must not
	// hand it a fresh retry budget.
	_, err := q.db.Exec(`UPDATE convoy.queue_jobs SET retry_count = 3 WHERE id = $1`, id)
	require.NoError(t, err)

	require.NoError(t, q.RunTask(ctx, name, id))
	var retryCount int
	require.NoError(t, q.db.Get(&retryCount, `SELECT retry_count FROM convoy.queue_jobs WHERE id = $1`, id))
	require.Equal(t, 3, retryCount)

	claimed, err := q.Claim(ctx, []string{name}, 10)
	require.NoError(t, err)
	require.Len(t, claimed, 1, "the task is due now")

	// A claimed row belongs to a live worker either way.
	require.ErrorIs(t, q.RunTask(ctx, name, id), queue.ErrTaskStatusConflict)
	require.ErrorIs(t, q.DeleteTask(ctx, name, id), queue.ErrTaskStatusConflict)

	require.NoError(t, q.Release(ctx, []string{id}))
	require.NoError(t, q.DeleteTask(ctx, name, id))
	require.ErrorIs(t, q.DeleteTask(ctx, name, id), queue.ErrTaskNotFound)
}

// A selection that has gone stale reports which rows moved and why the rest did
// not, rather than a count that leaves the operator guessing.
func TestBulkActionReportsPerTask(t *testing.T) {
	q := setupQueue(t)
	ctx := t.Context()
	name := string(convoy.EventQueue)

	ids := make([]string, 3)
	for i := range ids {
		ids[i] = ulid.Make().String()
		require.NoError(t, q.Write(ctx, convoy.EventProcessor, convoy.EventQueue, &queue.Job{ID: ids[i], Payload: []byte("x")}))
	}
	// One row moved out from under the selection.
	require.NoError(t, q.ArchiveTask(ctx, name, ids[2]))

	result, err := q.BulkAction(ctx, name, queue.ActionArchive, append(ids, "no-such-task"))
	require.NoError(t, err)
	require.Equal(t, 2, result.Succeeded)
	require.Len(t, result.Failures, 2)
	require.Contains(t, result.Failures, ids[2])
	require.Contains(t, result.Failures, "no-such-task")

	require.Equal(t, statusArchived, taskStatus(t, q, ids[0]))
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

// The postgres scheduler keeps its entries in the agent's memory, so the table
// is the only place the API can read them from.
func TestSchedulerEntriesRoundTrip(t *testing.T) {
	q := setupQueue(t)
	ctx := t.Context()
	next := time.Now().UTC().Add(time.Hour).Truncate(time.Second)

	require.NoError(t, q.RecordSchedulerEntry(ctx, queue.SchedulerEntry{
		ID:       "retention",
		Spec:     "0 1 * * *",
		TaskName: string(convoy.RetentionPolicies),
		Queue:    string(convoy.ScheduleQueue),
		NextRun:  &next,
	}))
	require.NoError(t, q.RecordSchedulerEntry(ctx, queue.SchedulerEntry{
		ID:       "removed",
		Spec:     "* * * * *",
		TaskName: "gone",
		Queue:    string(convoy.ScheduleQueue),
	}))

	entries, err := q.SchedulerEntries(ctx)
	require.NoError(t, err)
	require.Len(t, entries, 2)

	// Re-registering the same id updates it rather than duplicating it, since
	// every replica registers the same set on boot.
	prev := time.Now().UTC().Truncate(time.Second)
	require.NoError(t, q.RecordSchedulerEntry(ctx, queue.SchedulerEntry{
		ID:       "retention",
		Spec:     "0 2 * * *",
		TaskName: string(convoy.RetentionPolicies),
		Queue:    string(convoy.ScheduleQueue),
		NextRun:  &next,
		PrevRun:  &prev,
	}))

	// A task dropped from the code stops being advertised as scheduled.
	require.NoError(t, q.PruneSchedulerEntries(ctx, []string{"retention"}))

	entries, err = q.SchedulerEntries(ctx)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "retention", entries[0].ID)
	require.Equal(t, "0 2 * * *", entries[0].Spec)
	require.NotNil(t, entries[0].PrevRun)

	// An empty keep set is a process that registered nothing, which says
	// nothing about what another replica registered.
	require.NoError(t, q.PruneSchedulerEntries(ctx, nil))
	entries, err = q.SchedulerEntries(ctx)
	require.NoError(t, err)
	require.Len(t, entries, 1)
}

// The stats table gains a row per queue per day forever, so the nightly pass
// that clears archived rows clears buckets past the chart window too.
func TestDeleteArchivedPrunesOldStats(t *testing.T) {
	q := setupQueue(t)
	ctx := t.Context()
	name := string(convoy.EventQueue)

	_, err := q.db.Exec(`
		INSERT INTO convoy.queue_job_stats (queue_name, day, processed)
		VALUES ($1, (NOW() AT TIME ZONE 'UTC')::date - make_interval(days => $2::integer), 5),
		       ($1, (NOW() AT TIME ZONE 'UTC')::date, 7)`,
		name, queue.MaxHistoryDays+1)
	require.NoError(t, err)

	require.NoError(t, q.DeleteArchived(ctx))

	var days []string
	require.NoError(t, q.db.Select(&days, `
		SELECT day::text FROM convoy.queue_job_stats WHERE queue_name = $1`, name))
	require.Len(t, days, 1, "the bucket past the window is gone")
}
