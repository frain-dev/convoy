package driver

import (
	"context"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/internal/telemetry"
	"github.com/frain-dev/convoy/queue"
	"github.com/hibiken/asynq"
)

// QueueWorker provides a provider-agnostic worker API for registering
// and running task handlers.
type QueueWorker interface {
	RegisterHandlers(taskName convoy.TaskName, handler func(context.Context, *asynq.Task) error, tel *telemetry.Telemetry)
	Start()
	Stop()
}

type QueueDriver interface {
	Queuer() queue.Queuer
	Worker() QueueWorker
	Name() string
	Initialize() error
}
