package task

import (
	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/vmihailenco/taskq/v3"
)

func CreateTask(name convoy.TaskName, cfg config.Configuration, handler interface{}) *taskq.Task {

	options := taskq.TaskOptions{
		Name:       string(name),
		RetryLimit: int(cfg.Strategy.Default.RetryLimit),
		Handler:    handler,
	}

	return taskq.RegisterTask(&options)
}
