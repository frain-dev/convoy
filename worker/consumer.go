package worker

import (
	"context"
	"errors"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/worker/task"
	"github.com/hibiken/asynq"
	log "github.com/sirupsen/logrus"
)

type Consumer struct {
	queues map[string]int
	mux    *asynq.ServeMux
	srv    *asynq.Server
}

func NewConsumer(cfg config.Configuration, queues map[string]int) (*Consumer, error) {
	if cfg.Queue.Type != config.RedisQueueProvider {
		return nil, errors.New("please select the redis driver in your config")
	}

	dsn := cfg.Queue.Redis.Dsn
	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: dsn},
		asynq.Config{
			Concurrency: convoy.Concurrency,
			Queues:      queues,
			IsFailure: func(err error) bool {
				return err != task.ErrRateLimit
			},
			RetryDelayFunc: task.GetRetryDelay,
		},
	)

	mux := asynq.NewServeMux()

	return &Consumer{
		queues: queues,
		mux:    mux,
		srv:    srv,
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
