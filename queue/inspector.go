package queue

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Task statuses the monitor understands. A provider serves the subset its
// broker actually has: postgres has no scheduled or retry set, because a
// delivery waiting on a backoff is a pending row with a future run_at.
// Stats.Statuses names the live subset so the dashboard renders what exists
// rather than a fixed union with empty columns.
const (
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusScheduled  = "scheduled"
	StatusRetry      = "retry"
	StatusArchived   = "archived"
)

// Actions an operator can run on one task. Which apply depends on the provider
// and the status the task is in, so each provider decides per task rather than
// the dashboard re-deriving the rule: a button the broker will reject teaches
// an operator the wrong thing about the queue.
//
// Retry and Run are the same transition to the broker and differ only in what
// the operator is doing: Retry puts a task that already gave up back in the
// queue, Run pulls a task that is waiting on a backoff forward to now. They
// stay separate so the label matches the intent.
const (
	ActionRetry   = "retry"
	ActionRun     = "run"
	ActionArchive = "archive"
	ActionDelete  = "delete"
)

// TasksPerPage is the default page size. Both providers page by offset with no
// total: counting archived rows grows with retention, and the page only needs
// to know whether another one exists.
const TasksPerPage = 50

// MaxTasksPerPage bounds a caller-chosen page size, and with it the size of a
// bulk action. Each row costs a scan on postgres and a round trip on redis, so
// the number an operator picks is capped rather than trusted.
const MaxTasksPerPage = 100

// MaxTaskPage bounds how deep the drill-down pages. Every page costs a scan of
// the rows before it, and an unbounded page number overflows the offset into a
// negative one, so requests past this are rejected instead of served.
const MaxTaskPage = 1000

// MaxHistoryDays bounds the processed/failed series. Both providers keep daily
// buckets, so a longer window is more rows for a chart nobody reads that far
// back on.
const MaxHistoryDays = 30

// DefaultHistoryDays is the window the dashboard asks for when it does not say.
const DefaultHistoryDays = 7

// QueueStat is one queue's depth, keyed by the statuses its provider serves,
// plus the operational numbers that are not a count of queued work.
type QueueStat struct {
	Queue  string           `json:"queue"`
	Counts map[string]int64 `json:"counts"`
	// Paused means the workers are leaving this queue's work where it is.
	// Nothing is lost while paused; nothing moves either.
	Paused bool `json:"paused"`
	// LatencyMS is the age of the oldest task that is due but not yet claimed,
	// which is how far behind the workers are on this queue. A task scheduled
	// for later is not late, so it does not count.
	LatencyMS int64 `json:"latency_ms"`
	// Processed and Failed are today's counts, in the broker's own timezone
	// bucket (UTC). They are the head of the same series History returns.
	Processed int64 `json:"processed"`
	Failed    int64 `json:"failed"`
}

// Stats is the monitor's landing view. Statuses is the provider's vocabulary in
// display order and is the only thing a caller should build columns or filters
// from; hardcoding a status the running provider does not have produces a
// filter that can never match.
type Stats struct {
	Provider string      `json:"provider"`
	Statuses []string    `json:"statuses"`
	Queues   []QueueStat `json:"queues"`
}

// HistoryPoint is one UTC day of throughput for one queue. Failed counts every
// attempt that returned an error, so a task that fails twice and then succeeds
// contributes two failures and one processed, matching what asynq records.
type HistoryPoint struct {
	Date      string `json:"date"`
	Processed int64  `json:"processed"`
	Failed    int64  `json:"failed"`
}

// SchedulerEntry is one registered periodic task. Next and Prev are optional
// because a scheduler that has not ticked yet has no previous run, and an entry
// read from a registry the running process did not register has no next one.
type SchedulerEntry struct {
	ID       string     `json:"id"`
	Spec     string     `json:"spec"`
	TaskName string     `json:"task_name"`
	Queue    string     `json:"queue"`
	NextRun  *time.Time `json:"next_run_at,omitempty"`
	PrevRun  *time.Time `json:"prev_run_at,omitempty"`
}

// Task is one row of the drill-down. Payload and headers are deliberately
// absent: they carry customer event data, and this is an operational view of
// what is queued rather than an event browser.
//
// The optional times are optional because the providers know different things.
// A nil is "this provider does not track it", not zero.
type Task struct {
	ID         string     `json:"id"`
	Queue      string     `json:"queue"`
	TaskName   string     `json:"task_name"`
	Status     string     `json:"status"`
	RetryCount int        `json:"retry_count"`
	MaxRetry   int        `json:"max_retry"`
	NextRunAt  *time.Time `json:"next_run_at,omitempty"`
	ClaimedAt  *time.Time `json:"claimed_at,omitempty"`
	LastError  string     `json:"last_error,omitempty"`
	CreatedAt  *time.Time `json:"created_at,omitempty"`
	// Actions this task accepts right now. Empty means the task is not an
	// operator's to move: a live claim, or a scheduler row.
	Actions []string `json:"actions"`
}

// TaskFilter selects one page of the drill-down. Queue is required: asynq lists
// per queue, so an all-queues page would fan out one call per queue on redis
// and read worse on postgres, and the dashboard always arrives from a queue row.
//
// Search is an exact task id. Neither broker can scan for a partial id without
// reading every task, so a partial match would be a promise only one provider
// could keep.
type TaskFilter struct {
	Queue    string
	Status   string
	Page     int
	PageSize int
	Search   string
}

// Size is the page size to use, defaulted and bounded. Providers call this
// rather than reading PageSize, so a caller that omits it and one that sends
// something absurd land on the same rules.
func (f TaskFilter) Size() int {
	if f.PageSize < 1 {
		return TasksPerPage
	}
	if f.PageSize > MaxTasksPerPage {
		return MaxTasksPerPage
	}
	return f.PageSize
}

// TaskPage is one page of the drill-down. HasNext is computed exactly by each
// provider rather than inferred from a short page, so the last page does not
// depend on the page size dividing the total.
type TaskPage struct {
	Tasks   []Task `json:"tasks"`
	Page    int    `json:"page"`
	HasNext bool   `json:"has_next"`
}

// BulkResult reports what a bulk action actually moved. Succeeded plus the
// length of Failures is the number of ids the caller sent: an id the broker
// refused is named rather than folded into a count, because the operator
// selected those rows individually and is owed a reason for each one.
type BulkResult struct {
	Succeeded int               `json:"succeeded"`
	Failures  map[string]string `json:"failures,omitempty"`
}

// SchedulerRegistry is implemented by providers whose scheduler entries are not
// already readable by another process. Redis does not need it: asynq's own
// scheduler publishes its entries to redis, where the API can read them. The
// postgres scheduler keeps its entries in the agent's memory, so it publishes
// them here or the monitor has nothing to show.
type SchedulerRegistry interface {
	RecordSchedulerEntry(ctx context.Context, entry SchedulerEntry) error
	// PruneSchedulerEntries drops registered entries this process did not
	// register, so a task removed from the code stops being advertised as
	// scheduled. Every replica registers the same set, so one replica pruning
	// what another wrote is not a race between disagreeing owners.
	PruneSchedulerEntries(ctx context.Context, keepIDs []string) error
}

// TaskMutator is the per-task half of Inspector, which is all a bulk action
// needs. It exists so RunBulkAction can be shared by both providers without
// either of them being able to recurse into its own BulkAction.
type TaskMutator interface {
	RetryTask(ctx context.Context, queueName, taskID string) error
	RunTask(ctx context.Context, queueName, taskID string) error
	ArchiveTask(ctx context.Context, queueName, taskID string) error
	DeleteTask(ctx context.Context, queueName, taskID string) error
}

// RunBulkAction applies one action to each selected id, in order, collecting
// per-id outcomes. It does not stop at the first refusal: the operator selected
// these rows together, and a task that moved out from under the selection is
// expected rather than exceptional, so the rest still run.
//
// A transport failure is different. It says nothing about the remaining ids and
// probably means the next one fails too, so it aborts and is returned as an
// error, with the outcomes so far, rather than being recorded per id as though
// the broker had rejected each one.
func RunBulkAction(ctx context.Context, m TaskMutator, queueName, action string, taskIDs []string) (BulkResult, error) {
	if queueName == "" {
		return BulkResult{}, ErrQueueRequired
	}
	if len(taskIDs) == 0 {
		return BulkResult{}, ErrNoTaskIDs
	}
	if len(taskIDs) > MaxTasksPerPage {
		return BulkResult{}, fmt.Errorf("%w: at most %d", ErrTooManyTaskIDs, MaxTasksPerPage)
	}

	var run func(context.Context, string, string) error
	switch action {
	case ActionRetry:
		run = m.RetryTask
	case ActionRun:
		run = m.RunTask
	case ActionArchive:
		run = m.ArchiveTask
	case ActionDelete:
		run = m.DeleteTask
	default:
		return BulkResult{}, fmt.Errorf("%w: %q", ErrUnknownTaskAction, action)
	}

	result := BulkResult{}
	for _, id := range taskIDs {
		err := run(ctx, queueName, id)
		switch {
		case err == nil:
			result.Succeeded++
		case isTaskRefusal(err):
			if result.Failures == nil {
				result.Failures = map[string]string{}
			}
			result.Failures[id] = err.Error()
		default:
			return result, err
		}
	}
	return result, nil
}

// isTaskRefusal reports whether the broker decided about this one task, as
// opposed to failing to answer. Only a decision may be recorded against an id;
// anything else is the caller's to retry.
func isTaskRefusal(err error) bool {
	return errors.Is(err, ErrTaskNotFound) ||
		errors.Is(err, ErrTaskStatusConflict) ||
		errors.Is(err, ErrCronTaskImmutable)
}

// Inspector is the queue monitoring contract the dashboard talks to. Both
// providers implement all of it, so the page renders the same way on either
// broker and no caller has to type-assert for a capability.
type Inspector interface {
	Stats(ctx context.Context) (Stats, error)
	// History returns one point per UTC day, oldest first, including days with
	// no throughput, so a chart does not have to fill its own gaps.
	History(ctx context.Context, queueName string, days int) ([]HistoryPoint, error)
	SchedulerEntries(ctx context.Context) ([]SchedulerEntry, error)
	Tasks(ctx context.Context, filter TaskFilter) (TaskPage, error)

	// RetryTask returns one task to the queue. Both arguments are required:
	// asynq addresses a task by queue and id, and scoping the postgres update
	// by queue too keeps an operator from acting on a row they did not see.
	RetryTask(ctx context.Context, queueName, taskID string) error
	// RunTask pulls a task that is waiting on a backoff forward to now.
	RunTask(ctx context.Context, queueName, taskID string) error
	ArchiveTask(ctx context.Context, queueName, taskID string) error
	DeleteTask(ctx context.Context, queueName, taskID string) error
	// BulkAction runs one action over the ids an operator selected. It reports
	// per-id outcomes instead of stopping at the first refusal, because the
	// rows were selected together and a task that moved under the selection is
	// expected rather than exceptional.
	BulkAction(ctx context.Context, queueName, action string, taskIDs []string) (BulkResult, error)

	// PauseQueue stops workers claiming from this queue. Work already claimed
	// runs to completion; nothing new is picked up until it is resumed.
	PauseQueue(ctx context.Context, queueName string) error
	UnpauseQueue(ctx context.Context, queueName string) error
}
