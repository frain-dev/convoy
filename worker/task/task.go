package task

import (
	"github.com/frain-dev/convoy/config"
	"github.com/vmihailenco/taskq/v3"
)

func CreateTask(name string, cfg config.Configuration, handler interface{}) *taskq.Task {

	options := taskq.TaskOptions{
		Name:       name,
		RetryLimit: int(cfg.Strategy.Default.RetryLimit),
		Handler:    handler,
	}

	return taskq.RegisterTask(&options)
}
