package redis

import (
	"context"
	"errors"
	"fmt"
	"sort"

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

	list, ok := q.taskLister(f.Status)
	if !ok {
		return queue.TaskPage{}, fmt.Errorf("%w: %q", queue.ErrUnknownTaskStatus, f.Status)
	}

	infos, err := list(f.Queue, asynq.Page(f.Page), asynq.PageSize(queue.TasksPerPage))
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
		HasNext: f.Page*queue.TasksPerPage < statusCount(info, f.Status),
	}
	for _, i := range infos {
		page.Tasks = append(page.Tasks, task(i, f.Status))
	}
	return page, nil
}

// RetryTask moves a scheduled, retry or archived task back to pending.
func (q *RedisQueue) RetryTask(_ context.Context, queueName, taskID string) error {
	return inspectorError(q.inspector.RunTask(queueName, taskID))
}

// ArchiveTask takes a pending, scheduled or retry task out of the queue.
func (q *RedisQueue) ArchiveTask(_ context.Context, queueName, taskID string) error {
	return inspectorError(q.inspector.ArchiveTask(queueName, taskID))
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
		return []string{queue.ActionArchive}
	case queue.StatusScheduled, queue.StatusRetry:
		return []string{queue.ActionRetry, queue.ActionArchive}
	case queue.StatusArchived:
		return []string{queue.ActionRetry}
	default:
		return []string{}
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
