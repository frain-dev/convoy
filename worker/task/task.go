package task

import (
	"strings"

	"github.com/frain-dev/convoy"
	"github.com/vmihailenco/taskq/v3"
)

func CreateTask(t convoy.TaskName, group convoy.Group, handler interface{}) *taskq.Task {
	var name strings.Builder

	name.WriteString(group.Name)
	name.WriteString("-")
	name.WriteString(string(t))

	options := taskq.TaskOptions{
		Name:       name.String(),
		RetryLimit: int(group.Config.Strategy.Default.RetryLimit),
		Handler:    handler,
	}

	return taskq.RegisterTask(&options)
}
