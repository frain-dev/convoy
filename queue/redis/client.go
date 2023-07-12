package redis

import (
	"fmt"
	"github.com/danvixent/asynqmon"
	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/queue"
	"github.com/hibiken/asynq"
	"github.com/oklog/ulid/v2"
	"github.com/redis/go-redis/v9"
)

var (
	ErrTaskNotFound  = fmt.Errorf("asynq: %w", asynq.ErrTaskNotFound)
	ErrQueueNotFound = fmt.Errorf("asynq: %w", asynq.ErrQueueNotFound)
)

type RedisQueue struct {
	opts      queue.QueueOptions
	client    *asynq.Client
	inspector *asynq.Inspector
}

func NewQueue(opts queue.QueueOptions) queue.Queuer {
	var c asynq.RedisConnOpt
	if len(opts.RedisClusterAddresses) == 0 {
		var _ = opts.RedisClient.MakeRedisClient().(redis.UniversalClient)
		c = opts.RedisClient
	} else {
		c = asynq.RedisClusterClientOpt{
			Addrs: opts.RedisClusterAddresses,
		}
	}

	client := asynq.NewClient(c)
	inspector := asynq.NewInspector(c)
	return &RedisQueue{
		client:    client,
		opts:      opts,
		inspector: inspector,
	}
}

func (q *RedisQueue) Write(taskName convoy.TaskName, queueName convoy.QueueName, job *queue.Job) error {
	s := string(queueName)
	if job.ID == "" {
		job.ID = ulid.Make().String()
	}
	t := asynq.NewTask(string(taskName), job.Payload, asynq.Queue(s), asynq.TaskID(job.ID), asynq.ProcessIn(job.Delay))

	_, err := q.inspector.GetTaskInfo(s, job.ID)
	if err != nil {
		// If the task or queue does not yet exist, we can proceed
		// to enqueuing the task
		message := err.Error()
		if ErrQueueNotFound.Error() == message || ErrTaskNotFound.Error() == message {
			_, err := q.client.Enqueue(t, nil)
			return err
		}

		return err
	}

	// At this point, the task is already on the queue based on its ID.
	// We need to delete before enqueuing
	err = q.inspector.DeleteTask(s, job.ID)
	if err != nil {
		return err
	}

	_, err = q.client.Enqueue(t, nil)
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
