package redis

import (
	"errors"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
	"github.com/frain-dev/convoy/queue"
	"github.com/hibiken/asynq"
	"github.com/hibiken/asynqmon"
	"github.com/oklog/ulid/v2"
)

type RedisQueue struct {
	opts      queue.QueueOptions
	client    *asynq.Client
	inspector *asynq.Inspector
}

func NewClient(cfg config.Configuration) (*asynq.Client, error) {
	if cfg.Queue.Type != config.RedisQueueProvider {
		return nil, errors.New("please select the redis driver in your config")
	}

	redis, err := rdb.NewClient(cfg.Queue.BuildDsn())
	if err != nil {
		return nil, err
	}

	client := asynq.NewClient(redis)

	return client, nil
}

func NewQueue(opts queue.QueueOptions) queue.Queuer {
	client := asynq.NewClient(opts.RedisClient)
	inspector := asynq.NewInspector(opts.RedisClient)
	return &RedisQueue{
		client:    client,
		opts:      opts,
		inspector: inspector,
	}
}

func (q *RedisQueue) Write(taskName convoy.TaskName, queueName convoy.QueueName, job *queue.Job) error {
	if job.ID == "" {
		job.ID = ulid.Make().String()
	}
	t := asynq.NewTask(string(taskName), job.Payload, asynq.Queue(string(queueName)), asynq.TaskID(job.ID), asynq.ProcessIn(job.Delay))
	_, err := q.client.Enqueue(t)
	return err
}

func (q *RedisQueue) Options() queue.QueueOptions {
	return q.opts
}

func (q *RedisQueue) Monitor() *asynqmon.HTTPHandler {
	h := asynqmon.New(asynqmon.Options{
		RootPath:          "/queue/monitoring",
		RedisConnOpt:      q.opts.RedisClient,
		PrometheusAddress: q.opts.PrometheusAddress,
	})
	return h
}

func (q *RedisQueue) Inspector() *asynq.Inspector {
	return q.inspector
}

func (q *RedisQueue) DeleteEventDeliveriesFromQueue(queueName convoy.QueueName, ids []string) error {
	for _, id := range ids {
		taskInfo, err := q.inspector.GetTaskInfo(string(queueName), id)
		if err != nil {
			return err
		}
		if taskInfo.State == asynq.TaskStateActive {
			err = q.inspector.CancelProcessing(id)
			if err != nil {
				return err
			}
		}
		err = q.inspector.DeleteTask(string(queueName), id)
		if err != nil {
			return err
		}
	}
	return nil
}
