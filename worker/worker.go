package worker

import (
	"context"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/queue"
)

type Worker interface {
	StartAll(context.Context)
	StopAll() error
	Publish(context.Context, convoy.TaskName, convoy.QueueName, *queue.Job) error
}
