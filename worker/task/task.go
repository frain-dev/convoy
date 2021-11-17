package task

import (
	"github.com/frain-dev/convoy"
	"github.com/vmihailenco/taskq/v3"
)

func CreateTask(name convoy.TaskName, group convoy.Group, handler interface{}) *taskq.Task {

	options := taskq.TaskOptions{
		Name:       string(name),
		RetryLimit: int(group.Config.Strategy.Default.RetryLimit),
		Handler:    handler,
	}

	return taskq.RegisterTask(&options)
}
