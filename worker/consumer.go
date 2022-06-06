package worker

import (
	"context"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/worker/task"
	"github.com/hibiken/asynq"
	log "github.com/sirupsen/logrus"
)

type Consumer struct {
	queue queue.Queuer
	mux   *asynq.ServeMux
	srv   *asynq.Server
}

func NewConsumer(q queue.Queuer) (*Consumer, error) {
	dsn := q.Options().RedisAddress
	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: dsn},
		asynq.Config{
			Concurrency: convoy.Concurrency,
			Queues:      q.Options().Names,
			IsFailure: func(err error) bool {
				if _, ok := err.(*task.RateLimitError); ok {
					return false
				}
				return true
			},
			RetryDelayFunc: task.GetRetryDelay,
		},
	)

	mux := asynq.NewServeMux()

	return &Consumer{
		queue: q,
		mux:   mux,
		srv:   srv,
	}, nil
}

func (c *Consumer) Start() {
	if err := c.srv.Start(c.mux); err != nil {
		log.WithError(err).Fatal("error starting worker")
	}
}

func (c *Consumer) RegisterHandlers(taskName convoy.TaskName, handler func(context.Context, *asynq.Task) error) {
	c.mux.HandleFunc(string(taskName), handler)
}

func (c *Consumer) Stop() {
	c.srv.Stop()
	c.srv.Shutdown()
}
