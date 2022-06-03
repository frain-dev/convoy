package redis

import (
	"errors"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/util"
	"github.com/hibiken/asynq"
	"github.com/hibiken/asynqmon"
)

type RedisQueue struct {
	opts queue.QueueOptions
}

func NewClient(cfg config.Configuration) (*asynq.Client, error) {
	if cfg.Queue.Type != config.RedisQueueProvider {
		return nil, errors.New("please select the redis driver in your config")
	}

	dsn := cfg.Queue.Redis.Dsn
	if util.IsStringEmpty(dsn) {
		return nil, errors.New("please provide the Redis DSN")
	}

	client := asynq.NewClient(asynq.RedisClientOpt{Addr: dsn})

	return client, nil
}

func NewQueue(opts queue.QueueOptions) queue.Queuer {
	return &RedisQueue{
		opts: opts,
	}
}

func (q *RedisQueue) Write(taskName convoy.TaskName, queueName convoy.QueueName, job *queue.Job) error {
	t := asynq.NewTask(string(taskName), job.Payload, asynq.Queue(string(queueName)), asynq.ProcessIn(job.Delay))
	_, err := q.opts.Client.Enqueue(t)
	return err
}

func (q *RedisQueue) Options() queue.QueueOptions {
	return q.opts
}

func (q *RedisQueue) Telemetry() *asynqmon.HTTPHandler {
	h := asynqmon.New(asynqmon.Options{
		RootPath: "/queue/monitoring",
		RedisConnOpt: asynq.RedisClientOpt{
			Addr:     q.opts.Redis,
			Password: "",
			DB:       0,
		},
	})
	return h
}
