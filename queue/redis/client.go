package redis

import (
	"errors"

	"github.com/go-redis/redis/v8"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/util"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/hibiken/asynqmon"
)

type RedisQueue struct {
	opts      queue.QueueOptions
	inspector *asynq.Inspector
}

func NewClient(cfg config.Configuration) (*asynq.Client, error) {
	if cfg.Queue.Type != config.RedisQueueProvider {
		return nil, errors.New("please select the redis driver in your config")
	}

	if util.IsStringEmpty(cfg.Queue.Redis.Dsn) {
		return nil, errors.New("please provide the Redis DSN")
	}
	opts, err := redis.ParseURL(cfg.Queue.Redis.Dsn)
	if err != nil {
		return nil, errors.New("error parsing redis dsn")
	}

	client := asynq.NewClient(asynq.RedisClientOpt{Addr: opts.Addr})

	return client, nil
}

func NewQueue(opts queue.QueueOptions) queue.Queuer {
	rOpts, _ := redis.ParseURL(opts.RedisAddress)
	opts.RedisAddress = rOpts.Addr

	inspector := asynq.NewInspector(asynq.RedisClientOpt{
		Addr: opts.RedisAddress,
	})
	return &RedisQueue{
		opts:      opts,
		inspector: inspector,
	}
}

func (q *RedisQueue) Write(taskName convoy.TaskName, queueName convoy.QueueName, job *queue.Job) error {
	if job.ID == "" {
		job.ID = uuid.NewString()
	}
	t := asynq.NewTask(string(taskName), job.Payload, asynq.Queue(string(queueName)), asynq.TaskID(job.ID), asynq.ProcessIn(job.Delay))
	_, err := q.opts.Client.Enqueue(t)
	return err
}

func (q *RedisQueue) Options() queue.QueueOptions {
	return q.opts
}

func (q *RedisQueue) Monitor() *asynqmon.HTTPHandler {
	h := asynqmon.New(asynqmon.Options{
		RootPath: "/queue/monitoring",
		RedisConnOpt: asynq.RedisClientOpt{
			Addr:     q.opts.RedisAddress,
			Password: "",
			DB:       0,
		},
		PrometheusAddress: q.opts.PrometheusAddress,
	})
	return h
}

func (q *RedisQueue) Inspector() *asynq.Inspector {
	return q.inspector
}

func (q *RedisQueue) DeleteEventDeliveriesfromQueue(queuename convoy.QueueName, ids []string) error {
	for _, id := range ids {
		taskInfo, err := q.inspector.GetTaskInfo(string(queuename), id)
		if taskInfo.State == asynq.TaskStateActive {
			err = q.inspector.CancelProcessing(id)
			if err != nil {
				return err
			}
		}
		if err != nil {
			return err
		}
		err = q.inspector.DeleteTask(string(queuename), id)
		if err != nil {
			return err
		}
	}
	return nil
}
