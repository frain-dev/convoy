package worker

import (
	"context"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/worker/task"
	"github.com/hibiken/asynq"
)

type Consumer struct {
	queue queue.Queuer
	mux   *asynq.ServeMux
	srv   *asynq.Server
	log   log.StdLogger
}

func NewConsumer(q queue.Queuer, l log.StdLogger) *Consumer {
	srv := asynq.NewServer(
		q.Options().RedisClient,
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
			Logger:         l,
		},
	)

	mux := asynq.NewServeMux()

	return &Consumer{
		queue: q,
		log:   l,
		mux:   mux,
		srv:   srv,
	}
}

func (c *Consumer) Start() {
	if err := c.srv.Start(c.mux); err != nil {
		c.log.WithError(err).Fatal("error starting worker")
	}
}

func (c *Consumer) RegisterHandlers(taskName convoy.TaskName, handler func(context.Context, *asynq.Task) error) {
	c.mux.HandleFunc(string(taskName), handler)
}

func (c *Consumer) Stop() {
	c.srv.Stop()
	c.srv.Shutdown()
}
