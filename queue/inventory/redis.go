package inventory

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/hibiken/asynq"
)

// RedisInspector intentionally excludes every Asynq mutation method.
type RedisInspector interface {
	Queues() ([]string, error)
	GetQueueInfo(string) (*asynq.QueueInfo, error)
	ListScheduledTasks(string, ...asynq.ListOption) ([]*asynq.TaskInfo, error)
	ListRetryTasks(string, ...asynq.ListOption) ([]*asynq.TaskInfo, error)
}

type Redis struct{ Inspector RedisInspector }

func (r Redis) Inspect(ctx context.Context) (Snapshot, error) {
	if r.Inspector == nil {
		return Snapshot{}, errors.New("queue store inspector is unavailable")
	}
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	names, err := r.Inspector.Queues()
	if err != nil {
		return Snapshot{}, err
	}
	sort.Strings(names)
	snapshot := Snapshot{Queues: []Queue{}}
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return Snapshot{}, err
		}
		info, readErr := r.Inspector.GetQueueInfo(name)
		// Even a disappearing queue makes this multi-read observation incomplete.
		// A later refresh can establish the new inventory; do not turn races into zero.
		if readErr != nil {
			return Snapshot{}, readErr
		}
		if info == nil {
			return Snapshot{}, errors.New("missing queue information")
		}
		var nextDue *time.Time
		for _, delayed := range []struct {
			count int
			list  func(string, ...asynq.ListOption) ([]*asynq.TaskInfo, error)
		}{{info.Scheduled, r.Inspector.ListScheduledTasks}, {info.Retry, r.Inspector.ListRetryTasks}} {
			if delayed.count == 0 {
				continue
			}
			tasks, err := delayed.list(name, asynq.PageSize(1))
			if err != nil {
				return Snapshot{}, err
			}
			if len(tasks) > 0 && !tasks[0].NextProcessAt.IsZero() && (nextDue == nil || tasks[0].NextProcessAt.Before(*nextDue)) {
				stamp := tasks[0].NextProcessAt.UTC()
				nextDue = &stamp
			}
		}
		snapshot.Queues = append(snapshot.Queues, Queue{NextDueAt: nextDue, Name: name, Paused: info.Paused, OldestDueAgeMS: info.Latency.Milliseconds(), Counts: Counts{
			Pending: int64(info.Pending), Processing: int64(info.Active), Scheduled: int64(info.Scheduled), Retry: int64(info.Retry),
			Aggregating: int64(info.Aggregating), Archived: int64(info.Archived), Completed: int64(info.Completed),
		}})
	}
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	snapshot.ObservedAt = time.Now().UTC()
	return snapshot, nil
}
