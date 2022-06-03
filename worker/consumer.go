package worker

import (
	"context"
	"errors"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/worker/task"
	"github.com/go-redis/redis/v8"
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
	rOpts, _ := redis.ParseURL(dsn)
	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: rOpts.Addr},
		asynq.Config{
			Concurrency: convoy.Concurrency,
			Queues:      queues,
			IsFailure: func(err error) bool {
				if _, ok := err.(*task.RateLimitError); ok {
					return true
				}
				return false
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
