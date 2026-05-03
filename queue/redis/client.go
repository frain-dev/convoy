package redis

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/danvixent/asynqmon"
	"github.com/hibiken/asynq"
	"github.com/oklog/ulid/v2"
	"github.com/redis/go-redis/v9"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/internal/pkg/queue/tracectx"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/frain-dev/convoy/queue"
)

var (
	ErrTaskNotFound  = fmt.Errorf("asynq: %w", asynq.ErrTaskNotFound)
	ErrQueueNotFound = fmt.Errorf("asynq: %w", asynq.ErrQueueNotFound)
)

type RedisQueue struct {
	opts      queue.QueueOptions
	client    *asynq.Client
	inspector *asynq.Inspector
	logger    log.Logger
}

func NewQueue(opts queue.QueueOptions) queue.Queuer {
	var c asynq.RedisConnOpt
	if opts.RedisFailoverOpt != nil {
		c = *opts.RedisFailoverOpt
	} else if opts.RedisClient != nil {
		c = opts.RedisClient
	} else if len(opts.RedisAddress) == 1 {
		var _ = opts.RedisClient.MakeRedisClient().(redis.UniversalClient)
		c = opts.RedisClient
	} else {
		c = asynq.RedisClusterClientOpt{
			Addrs: opts.RedisAddress,
		}
	}

	client := asynq.NewClient(c)
	inspector := asynq.NewInspector(c)
	return &RedisQueue{
		client:    client,
		opts:      opts,
		inspector: inspector,
		logger:    log.New("convoy", log.LevelInfo),
	}
}

func (q *RedisQueue) Write(ctx context.Context, taskName convoy.TaskName, queueName convoy.QueueName, job *queue.Job) error {
	s := string(queueName)
	if job.ID == "" {
		job.ID = ulid.Make().String()
	}
	// Inject the active OTel trace context into job.Headers so the worker
	// span on the consumer side becomes a child of the producer's. No-op
	// when ctx has no active span, so untraced callers stay zero-cost.
	tracectx.InjectIntoJob(ctx, job)
	t := asynq.NewTaskWithHeaders(string(taskName), job.Payload, job.Headers,
		asynq.Queue(s), asynq.TaskID(job.ID), asynq.ProcessIn(job.Delay))

	// Optimization: Try to enqueue directly first (optimistic path)
	// This reduces from 3 Redis calls to 1 in the common case (no duplicate)
	_, err := q.client.Enqueue(t, nil)
	if err == nil {
		return nil // Success - saved 2 Redis calls!
	}

	// If enqueue failed due to duplicate task ID, delete and retry
	// Check if it's a duplicate task error (Asynq returns this when task ID exists)
	if err == asynq.ErrDuplicateTask || err == asynq.ErrTaskIDConflict {
		// Delete the existing task and retry
		deleteErr := q.inspector.DeleteTask(s, job.ID)
		if deleteErr != nil {
			return deleteErr
		}
		_, err = q.client.Enqueue(t, nil)
		return err
	}

	// For other errors (queue not found, etc.), return as-is
	return err
}

func (q *RedisQueue) WriteWithoutTimeout(ctx context.Context, taskName convoy.TaskName, queueName convoy.QueueName, job *queue.Job) error {
	s := string(queueName)
	if job.ID == "" {
		job.ID = ulid.Make().String()
	}

	tracectx.InjectIntoJob(ctx, job)
	t := asynq.NewTaskWithHeaders(string(taskName), job.Payload, job.Headers,
		asynq.Queue(s), asynq.TaskID(job.ID), asynq.Timeout(0), asynq.ProcessIn(job.Delay))

	// Optimization: Try to enqueue directly first (optimistic path)
	// This reduces from 3 Redis calls to 1 in the common case (no duplicate)
	_, err := q.client.Enqueue(t, nil)
	if err == nil {
		return nil // Success - saved 2 Redis calls!
	}

	// If enqueue failed due to duplicate task ID, delete and retry
	// Check if it's a duplicate task error (Asynq returns this when task ID exists)
	if err == asynq.ErrDuplicateTask || err == asynq.ErrTaskIDConflict {
		// Delete the existing task and retry
		deleteErr := q.inspector.DeleteTask(s, job.ID)
		if deleteErr != nil {
			return deleteErr
		}
		_, err = q.client.Enqueue(t, nil)
		return err
	}

	// For other errors (queue not found, etc.), return as-is
	return err
}

func (q *RedisQueue) Options() queue.QueueOptions {
	return q.opts
}

func (q *RedisQueue) Monitor() *asynqmon.HTTPHandler {
	return q.MonitorWithRootPath("/queue/monitoring")
}

// MonitorWithRootPath builds an Asynqmon handler for a custom mount path.
func (q *RedisQueue) MonitorWithRootPath(rootPath string) *asynqmon.HTTPHandler {
	var redisConnOpt asynq.RedisConnOpt
	if q.opts.RedisFailoverOpt != nil {
		redisConnOpt = *q.opts.RedisFailoverOpt
	} else {
		redisConnOpt = q.opts.RedisClient
	}

	h := asynqmon.New(asynqmon.Options{
		RootPath:          rootPath,
		RedisConnOpt:      redisConnOpt,
		PrometheusAddress: q.opts.PrometheusAddress,
		PayloadFormatter:  Formatter{},
		ResultFormatter:   Formatter{},
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

type Formatter struct {
}

func (f Formatter) FormatPayload(_ string, payload []byte) string {
	var pack map[string]interface{}
	_ = msgpack.DecodeMsgPack(payload, &pack)
	bytes, _ := json.Marshal(pack)
	return string(bytes)
}

func (f Formatter) FormatResult(_ string, payload []byte) string {
	var pack map[string]interface{}
	_ = msgpack.DecodeMsgPack(payload, &pack)
	bytes, _ := json.Marshal(pack)
	return string(bytes)
}
