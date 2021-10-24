package task

import (
	"github.com/Subomi/taskq/v3"
	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
)

func CreateTask(name convoy.TaskName, cfg config.Configuration, handler interface{}) *taskq.Task {

	options := taskq.TaskOptions{
		Name:       string(name),
		RetryLimit: int(cfg.Strategy.Default.RetryLimit),
		Handler:    handler,
	}

	return taskq.RegisterTask(&options)
}
