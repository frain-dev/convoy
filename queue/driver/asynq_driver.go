package driver

import (
	"context"

	"github.com/hibiken/asynq"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/internal/telemetry"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/worker"
)

type AsynqWorker struct {
	consumer *worker.Consumer
}

func NewAsynqWorker(consumer *worker.Consumer) *AsynqWorker {
	return &AsynqWorker{consumer: consumer}
}

func (w *AsynqWorker) RegisterHandlers(taskName convoy.TaskName, handler func(context.Context, *asynq.Task) error, tel *telemetry.Telemetry) {
	w.consumer.RegisterHandlers(taskName, handler, tel)
}

func (w *AsynqWorker) Start() { w.consumer.Start() }
func (w *AsynqWorker) Stop()  { w.consumer.Stop() }

type AsynqDriver struct {
	name   string
	queuer queue.Queuer
	worker *AsynqWorker
}

func NewAsynqDriver(q queue.Queuer, ctx context.Context, consumerPoolSize int, lo log.StdLogger, level log.Level) *AsynqDriver {
	c := worker.NewConsumer(ctx, consumerPoolSize, q, lo, level)
	return &AsynqDriver{
		name:   "redis",
		queuer: q,
		worker: NewAsynqWorker(c),
	}
}

func (d *AsynqDriver) Queuer() queue.Queuer { return d.queuer }
func (d *AsynqDriver) Worker() QueueWorker  { return d.worker }
func (d *AsynqDriver) Name() string         { return d.name }
func (d *AsynqDriver) Initialize() error    { return nil }
