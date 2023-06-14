package redis

import (
	"fmt"
	"time"

	"github.com/danvixent/asynqmon"
	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/queue"
	"github.com/hibiken/asynq"
	"github.com/oklog/ulid/v2"
	"github.com/redis/go-redis/v9"
)

type RedisQueue struct {
	opts      queue.QueueOptions
	client    *asynq.Client
	inspector *asynq.Inspector
}

func NewQueue(opts queue.QueueOptions) queue.Queuer {
	var _ redis.UniversalClient = opts.RedisClient.MakeRedisClient().(redis.UniversalClient)

	client := asynq.NewClient(opts.RedisClient)
	inspector := asynq.NewInspector(opts.RedisClient)
	return &RedisQueue{
		client:    client,
		opts:      opts,
		inspector: inspector,
	}
}

func (q *RedisQueue) Write(taskName convoy.TaskName, queueName convoy.QueueName, job *queue.Job) error {
	queue := string(queueName)
	if job.ID == "" {
		job.ID = ulid.Make().String()
	}
	t := asynq.NewTask(string(taskName), job.Payload, asynq.Queue(queue), asynq.TaskID(job.ID), asynq.ProcessIn(job.Delay))

	_, err := q.inspector.GetTaskInfo(queue, job.ID)
	if err != nil {
		taskNotFound := fmt.Errorf("asynq: %w", asynq.ErrTaskNotFound)
		if taskNotFound.Error() == err.Error() {
			_, err := q.client.Enqueue(t, asynq.Retention(24*time.Hour))
			return err
		}

		return err
	}

	err = q.inspector.DeleteTask(queue, job.ID)
	if err != nil {
		return err
	}

	_, err = q.client.Enqueue(t, asynq.Retention(24*time.Hour))
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
