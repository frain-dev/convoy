package queue

import (
	"context"
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
const (
	ActionRetry   = "retry"
	ActionArchive = "archive"
)

// TasksPerPage bounds one drill-down page. Both providers page by offset with
// no total: counting archived rows grows with retention, and the page only
// needs to know whether another one exists.
const TasksPerPage = 50

// MaxTaskPage bounds how deep the drill-down pages. Every page costs a scan of
// the rows before it, and an unbounded page number overflows the offset into a
// negative one, so requests past this are rejected instead of served.
const MaxTaskPage = 1000

// QueueStat is one queue's depth, keyed by the statuses its provider serves.
type QueueStat struct {
	Queue  string           `json:"queue"`
	Counts map[string]int64 `json:"counts"`
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
type TaskFilter struct {
	Queue  string
	Status string
	Page   int
}

// TaskPage is one page of the drill-down. HasNext is computed exactly by each
// provider rather than inferred from a short page, so the last page does not
// depend on the page size dividing the total.
type TaskPage struct {
	Tasks   []Task `json:"tasks"`
	Page    int    `json:"page"`
	HasNext bool   `json:"has_next"`
}

// Inspector is the queue monitoring contract the dashboard talks to. Both
// providers implement all of it, so the page renders the same way on either
// broker and no caller has to type-assert for a capability.
type Inspector interface {
	Stats(ctx context.Context) (Stats, error)
	Tasks(ctx context.Context, filter TaskFilter) (TaskPage, error)
	// RetryTask returns one task to the queue. Both arguments are required:
	// asynq addresses a task by queue and id, and scoping the postgres update
	// by queue too keeps an operator from acting on a row they did not see.
	RetryTask(ctx context.Context, queueName, taskID string) error
	ArchiveTask(ctx context.Context, queueName, taskID string) error
}
