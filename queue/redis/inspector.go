package redis

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/hibiken/asynq"

	"github.com/frain-dev/convoy/queue"
)

var _ queue.Inspector = (*RedisQueue)(nil)

// inspectorStatuses is the vocabulary this provider serves, in the order the
// dashboard shows them. Asynq also keeps completed and aggregating sets;
// Convoy configures neither result retention nor task groups, so both are
// always empty and are left out rather than shown as columns that never move.
var inspectorStatuses = []string{
	queue.StatusPending,
	queue.StatusProcessing,
	queue.StatusScheduled,
	queue.StatusRetry,
	queue.StatusArchived,
}

// Stats reads every queue asynq knows about, not only the ones this replica is
// configured to consume, so a queue left behind by an execution-mode change is
// still visible to the operator who has to drain it.
func (q *RedisQueue) Stats(_ context.Context) (queue.Stats, error) {
	names, err := q.inspector.Queues()
	if err != nil {
		return queue.Stats{}, err
	}
	sort.Strings(names)

	stats := queue.Stats{
		Provider: queue.ProviderRedis,
		Statuses: inspectorStatuses,
		Queues:   make([]queue.QueueStat, 0, len(names)),
	}
	for _, name := range names {
		info, err := q.inspector.GetQueueInfo(name)
		if err != nil {
			// A queue can be deleted between the listing and this read. That is
			// the one absence we can prove, so it is skipped; any other failure
			// is returned rather than reported as a queue with no work in it.
			if errors.Is(err, asynq.ErrQueueNotFound) {
				continue
			}
			return queue.Stats{}, err
		}
		stats.Queues = append(stats.Queues, queue.QueueStat{
			Queue: name,
			Counts: map[string]int64{
				queue.StatusPending:    int64(info.Pending),
				queue.StatusProcessing: int64(info.Active),
				queue.StatusScheduled:  int64(info.Scheduled),
				queue.StatusRetry:      int64(info.Retry),
				queue.StatusArchived:   int64(info.Archived),
			},
			Paused: info.Paused,
			// asynq's Latency is the age of the oldest pending task, which is
			// the same "how far behind are the workers" the postgres provider
			// derives from run_at.
			LatencyMS: info.Latency.Milliseconds(),
			Processed: int64(info.Processed),
			Failed:    int64(info.Failed),
		})
	}
	return stats, nil
}

func (q *RedisQueue) Tasks(_ context.Context, f queue.TaskFilter) (queue.TaskPage, error) {
	if f.Queue == "" {
		return queue.TaskPage{}, queue.ErrQueueRequired
	}
	if f.Page < 1 || f.Page > queue.MaxTaskPage {
		return queue.TaskPage{}, fmt.Errorf("%w: page must be between 1 and %d", queue.ErrInvalidPage, queue.MaxTaskPage)
	}
	if f.Search != "" {
		return q.searchTask(f)
	}

	list, ok := q.taskLister(f.Status)
	if !ok {
		return queue.TaskPage{}, fmt.Errorf("%w: %q", queue.ErrUnknownTaskStatus, f.Status)
	}

	size := f.Size()
	infos, err := list(f.Queue, asynq.Page(f.Page), asynq.PageSize(size))
	if err != nil {
		if errors.Is(err, asynq.ErrQueueNotFound) {
			return queue.TaskPage{}, queue.ErrQueueNotFound
		}
		return queue.TaskPage{}, err
	}

	// Asynq derives the offset from the page size, so the extra-row probe the
	// postgres provider uses would shift every later page. The set's own size
	// answers the same question exactly.
	info, err := q.inspector.GetQueueInfo(f.Queue)
	if err != nil {
		return queue.TaskPage{}, err
	}

	page := queue.TaskPage{
		Page:    f.Page,
		Tasks:   make([]queue.Task, 0, len(infos)),
		HasNext: f.Page*size < statusCount(info, f.Status),
	}
	for _, i := range infos {
		page.Tasks = append(page.Tasks, task(i, f.Status))
	}
	return page, nil
}

// searchTask answers an exact id lookup. The status filter is deliberately not
// applied: an operator searching an id wants to know where that task is, and
// answering "not found" because it moved to another status would be a lie about
// the queue. The status comes from the task itself here rather than from the
// set it was listed under, because there is no listing.
func (q *RedisQueue) searchTask(f queue.TaskFilter) (queue.TaskPage, error) {
	page := queue.TaskPage{Page: 1, Tasks: []queue.Task{}}
	if f.Page > 1 {
		return page, nil
	}

	info, err := q.inspector.GetTaskInfo(f.Queue, f.Search)
	switch {
	case errors.Is(err, asynq.ErrTaskNotFound), errors.Is(err, asynq.ErrQueueNotFound):
		return page, nil
	case err != nil:
		return queue.TaskPage{}, err
	}

	page.Tasks = append(page.Tasks, task(info, taskState(info.State)))
	return page, nil
}

// History returns the daily throughput series asynq already keeps. Its buckets
// are UTC days, the same as the postgres provider's, so the two providers plot
// the same shape.
func (q *RedisQueue) History(_ context.Context, queueName string, days int) ([]queue.HistoryPoint, error) {
	if queueName == "" {
		return nil, queue.ErrQueueRequired
	}
	if days < 1 {
		days = queue.DefaultHistoryDays
	}
	if days > queue.MaxHistoryDays {
		days = queue.MaxHistoryDays
	}

	stats, err := q.inspector.History(queueName, days)
	if err != nil {
		if errors.Is(err, asynq.ErrQueueNotFound) {
			return nil, queue.ErrQueueNotFound
		}
		return nil, err
	}

	// asynq returns newest first; the chart reads left to right in time.
	points := make([]queue.HistoryPoint, 0, len(stats))
	for i := len(stats) - 1; i >= 0; i-- {
		s := stats[i]
		points = append(points, queue.HistoryPoint{
			Date:      s.Date.UTC().Format(time.DateOnly),
			Processed: int64(s.Processed),
			Failed:    int64(s.Failed),
		})
	}
	return points, nil
}

// SchedulerEntries reads asynq's own registry, which every scheduler replica
// writes to redis, so this works regardless of which process is asked.
func (q *RedisQueue) SchedulerEntries(_ context.Context) ([]queue.SchedulerEntry, error) {
	found, err := q.inspector.SchedulerEntries()
	if err != nil {
		return nil, err
	}

	entries := make([]queue.SchedulerEntry, 0, len(found))
	for _, e := range found {
		entry := queue.SchedulerEntry{
			ID:       e.ID,
			Spec:     e.Spec,
			TaskName: e.Task.Type(),
			Queue:    schedulerQueue(e.Opts),
		}
		if !e.Next.IsZero() {
			next := e.Next
			entry.NextRun = &next
		}
		if !e.Prev.IsZero() {
			prev := e.Prev
			entry.PrevRun = &prev
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].TaskName != entries[j].TaskName {
			return entries[i].TaskName < entries[j].TaskName
		}
		return entries[i].ID < entries[j].ID
	})
	return entries, nil
}

// schedulerQueue pulls the target queue out of the entry's options. asynq
// stores options as an opaque list, so the queue is read from the one option
// type that carries it rather than parsed out of the string form.
func schedulerQueue(opts []asynq.Option) string {
	for _, opt := range opts {
		if opt.Type() == asynq.QueueOpt {
			if name, ok := opt.Value().(string); ok {
				return name
			}
		}
	}
	return ""
}

// RetryTask moves a scheduled, retry or archived task back to pending.
func (q *RedisQueue) RetryTask(_ context.Context, queueName, taskID string) error {
	return inspectorError(q.inspector.RunTask(queueName, taskID))
}

// RunTask pulls a task waiting on a backoff forward to now. asynq expresses
// both this and a retry as the same move back to pending; they stay separate
// here so the dashboard can label each one for what the operator is doing.
func (q *RedisQueue) RunTask(_ context.Context, queueName, taskID string) error {
	return inspectorError(q.inspector.RunTask(queueName, taskID))
}

// ArchiveTask takes a pending, scheduled or retry task out of the queue.
func (q *RedisQueue) ArchiveTask(_ context.Context, queueName, taskID string) error {
	return inspectorError(q.inspector.ArchiveTask(queueName, taskID))
}

// DeleteTask drops a task for good. asynq refuses to delete an active task,
// which is the same rule the postgres provider enforces on a claimed row.
func (q *RedisQueue) DeleteTask(_ context.Context, queueName, taskID string) error {
	return inspectorError(q.inspector.DeleteTask(queueName, taskID))
}

func (q *RedisQueue) BulkAction(ctx context.Context, queueName, action string, taskIDs []string) (queue.BulkResult, error) {
	return queue.RunBulkAction(ctx, q, queueName, action, taskIDs)
}

// PauseQueue stops workers claiming from this queue. asynq enforces it inside
// its own processor, so nothing in Convoy has to check it.
func (q *RedisQueue) PauseQueue(_ context.Context, queueName string) error {
	return pauseError(q.inspector.PauseQueue(queueName))
}

func (q *RedisQueue) UnpauseQueue(_ context.Context, queueName string) error {
	return pauseError(q.inspector.UnpauseQueue(queueName))
}

// pauseError treats "already in that state" as success. The operator asked for
// an end state, not a transition, and reporting a queue that is already paused
// as an error would make a second click look like a failure. asynq expresses
// both no-ops as plain errors with no sentinel to match, so the two messages it
// uses are matched by text; anything else is a real broker failure.
func pauseError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, asynq.ErrQueueNotFound):
		return queue.ErrQueueNotFound
	case strings.Contains(err.Error(), "is already paused"), strings.Contains(err.Error(), "is not paused"):
		return nil
	default:
		return err
	}
}

// inspectorError maps the outcomes asynq classifies. It refuses a task or queue
// it cannot find, and everything else stays an unclassified error: asynq does
// not export the state-conflict case, and reporting a redis transport failure
// as a status conflict would tell an operator the action was rejected when it
// may never have been attempted.
func inspectorError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, asynq.ErrTaskNotFound):
		return queue.ErrTaskNotFound
	case errors.Is(err, asynq.ErrQueueNotFound):
		return queue.ErrTaskNotFound
	default:
		return err
	}
}

func (q *RedisQueue) taskLister(status string) (func(string, ...asynq.ListOption) ([]*asynq.TaskInfo, error), bool) {
	switch status {
	case queue.StatusPending:
		return q.inspector.ListPendingTasks, true
	case queue.StatusProcessing:
		return q.inspector.ListActiveTasks, true
	case queue.StatusScheduled:
		return q.inspector.ListScheduledTasks, true
	case queue.StatusRetry:
		return q.inspector.ListRetryTasks, true
	case queue.StatusArchived:
		return q.inspector.ListArchivedTasks, true
	default:
		return nil, false
	}
}

// actions mirrors the transitions asynq accepts, so the page only offers what
// the broker will take. An active task is held by a worker that will finish or
// retry it underneath either action, so it offers neither. A pending task is
// already runnable and an archived one is already out of the queue.
func actions(status string) []string {
	switch status {
	case queue.StatusPending:
		return []string{queue.ActionArchive, queue.ActionDelete}
	case queue.StatusScheduled, queue.StatusRetry:
		return []string{queue.ActionRun, queue.ActionArchive, queue.ActionDelete}
	case queue.StatusArchived:
		return []string{queue.ActionRetry, queue.ActionDelete}
	default:
		return []string{}
	}
}

// taskState maps asynq's own state back to the monitor's vocabulary. It is only
// used where there is no listing to take the status from, which is the id
// search: everywhere else the status is the set the task was read from, so a
// task that moves mid-listing is still reported under the column the operator
// is looking at. Completed and aggregating tasks are not states this monitor
// serves, and fall through to the empty string, which offers no actions.
func taskState(state asynq.TaskState) string {
	switch state {
	case asynq.TaskStatePending:
		return queue.StatusPending
	case asynq.TaskStateActive:
		return queue.StatusProcessing
	case asynq.TaskStateScheduled:
		return queue.StatusScheduled
	case asynq.TaskStateRetry:
		return queue.StatusRetry
	case asynq.TaskStateArchived:
		return queue.StatusArchived
	default:
		return ""
	}
}

func statusCount(info *asynq.QueueInfo, status string) int {
	switch status {
	case queue.StatusPending:
		return info.Pending
	case queue.StatusProcessing:
		return info.Active
	case queue.StatusScheduled:
		return info.Scheduled
	case queue.StatusRetry:
		return info.Retry
	case queue.StatusArchived:
		return info.Archived
	default:
		return 0
	}
}

// task maps one asynq task. Payload, headers and results are dropped: they
// carry customer event data, and this is an operational view of what is queued.
// The status is the set the task was listed from rather than TaskInfo.State, so
// a task that moves between the listing and this loop is still reported under
// the column the operator is looking at.
func task(i *asynq.TaskInfo, status string) queue.Task {
	t := queue.Task{
		ID:         i.ID,
		Queue:      i.Queue,
		TaskName:   i.Type,
		Status:     status,
		RetryCount: i.Retried,
		MaxRetry:   i.MaxRetry,
		LastError:  i.LastErr,
		Actions:    actions(status),
	}
	if !i.NextProcessAt.IsZero() {
		next := i.NextProcessAt
		t.NextRunAt = &next
	}
	return t
}
