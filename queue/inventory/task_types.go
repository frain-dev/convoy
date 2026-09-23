package inventory

import (
	"context"
	"sort"

	"github.com/hibiken/asynq"
)

type TaskType struct {
	Queue string `db:"queue_name"`
	Name  string `db:"task_name"`
}
type TaskTypeReader interface {
	TaskTypes(context.Context) ([]TaskType, error)
}

func (p Postgres) TaskTypes(ctx context.Context) ([]TaskType, error) {
	result := []TaskType{}
	err := p.DB.SelectContext(ctx, &result, `SELECT DISTINCT queue_name,task_name FROM convoy.queue_jobs WHERE status<>'completed' ORDER BY queue_name,task_name`)
	return result, err
}

func (r configuredRedis) TaskTypes(ctx context.Context) ([]TaskType, error) {
	seen := map[TaskType]bool{}
	err := r.withInspector(ctx, func(i *asynq.Inspector) error {
		names, err := i.Queues()
		if err != nil {
			return err
		}
		for _, name := range names {
			lists := []func(string, ...asynq.ListOption) ([]*asynq.TaskInfo, error){i.ListPendingTasks, i.ListActiveTasks, i.ListScheduledTasks, i.ListRetryTasks, i.ListArchivedTasks}
			scan := func(list func(...asynq.ListOption) ([]*asynq.TaskInfo, error)) error {
				for page := 1; ; page++ {
					if err := ctx.Err(); err != nil {
						return err
					}
					tasks, err := list(asynq.Page(page), asynq.PageSize(100))
					if err != nil {
						return err
					}
					for _, task := range tasks {
						seen[TaskType{Queue: name, Name: task.Type}] = true
					}
					if len(tasks) < 100 {
						return nil
					}
				}
			}
			for _, list := range lists {
				if err := scan(func(opts ...asynq.ListOption) ([]*asynq.TaskInfo, error) { return list(name, opts...) }); err != nil {
					return err
				}
			}
			groups, err := i.Groups(name)
			if err != nil {
				return err
			}
			for _, group := range groups {
				if err := scan(func(opts ...asynq.ListOption) ([]*asynq.TaskInfo, error) {
					return i.ListAggregatingTasks(name, group.Group, opts...)
				}); err != nil {
					return err
				}
			}
		}
		return nil
	})
	result := make([]TaskType, 0, len(seen))
	for kind := range seen {
		result = append(result, kind)
	}
	sort.Slice(result, func(a, b int) bool {
		if result[a].Queue == result[b].Queue {
			return result[a].Name < result[b].Name
		}
		return result[a].Queue < result[b].Queue
	})
	return result, err
}
