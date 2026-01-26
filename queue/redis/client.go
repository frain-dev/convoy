package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/oklog/ulid/v2"
	"github.com/olamilekan000/surge/surge"
	"github.com/olamilekan000/surge/surge/backend"
	"github.com/olamilekan000/surge/surge/config"
	"github.com/olamilekan000/surge/surge/errors"
	"github.com/olamilekan000/surge/surge/server"
	"github.com/redis/go-redis/v9"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/frain-dev/convoy/queue"
)

var (
	ErrTaskNotFound  = fmt.Errorf("surge: %w", errors.ErrJobNotFound)
	ErrQueueNotFound = fmt.Errorf("surge: %w", errors.ErrQueueNotFound)
)

type RedisQueue struct {
	opts    queue.QueueOptions
	client  *surge.Client
	backend backend.Backend
	ctx     context.Context
}

type TaskPayload struct {
	TaskName string `json:"task_name"`
	Payload  []byte `json:"payload"`
	ID       string `json:"id"`
	Queue    string `json:"queue"`
}

func (t TaskPayload) JobName() string {
	return t.TaskName
}

func extractNamespaceFromJobID(jobID string) string {
	parts := strings.Split(jobID, ":")
	if len(parts) >= 2 {
		return parts[1]
	}
	return "system"
}

func NewQueue(opts queue.QueueOptions) queue.Queuer {
	ctx := context.Background()

	maxWorkers := opts.MaxWorkers
	if maxWorkers <= 0 {
		maxWorkers = 50
	}

	cfg := &config.Config{
		MaxWorkers:       maxWorkers,
		DefaultNamespace: "default",
		RedisHost:        "localhost",
		RedisPort:        6379,
		RedisDB:          0,
	}

	if len(opts.RedisAddress) > 0 && opts.RedisAddress[0] != "" {
		redisOpts, err := redis.ParseURL(opts.RedisAddress[0])
		if err == nil {
			if redisOpts.Addr != "" {
				parts := strings.Split(redisOpts.Addr, ":")
				if len(parts) == 2 {
					if parts[0] != "" {
						cfg.RedisHost = parts[0]
					}
					if port, err := strconv.Atoi(parts[1]); err == nil {
						cfg.RedisPort = port
					}
				}
			}
			cfg.RedisPassword = redisOpts.Password
			cfg.RedisUsername = redisOpts.Username
			cfg.RedisDB = redisOpts.DB
		}
	}

	client, err := surge.NewClient(ctx, cfg)
	if err != nil {
		panic(fmt.Sprintf("failed to create surge client: %v", err))
	}

	return &RedisQueue{
		client:  client,
		backend: client.Backend(),
		opts:    opts,
		ctx:     ctx,
	}
}

func (q *RedisQueue) Write(taskName convoy.TaskName, queueName convoy.QueueName, job *queue.Job) error {
	if job.ID == "" {
		job.ID = ulid.Make().String()
	}

	namespace := extractNamespaceFromJobID(job.ID)

	payload := TaskPayload{
		TaskName: string(taskName),
		Payload:  job.Payload,
		ID:       job.ID,
		Queue:    string(queueName),
	}

	jobBuilder := q.client.Job(payload).Ns(namespace)

	if job.Delay > 0 {
		jobBuilder = jobBuilder.Schedule(job.Delay)
	}

	// jobBuilder = jobBuilder.UniqueFor(24 * time.Hour)

	return jobBuilder.Enqueue(q.ctx)
}

func (q *RedisQueue) WriteWithoutTimeout(taskName convoy.TaskName, queueName convoy.QueueName, job *queue.Job) error {
	return q.Write(taskName, queueName, job)
}

func (q *RedisQueue) Options() queue.QueueOptions {
	return q.opts
}

func (q *RedisQueue) Monitor() http.Handler {
	dashboard := server.NewDashboardServer(q.client, 0)
	dashboard.SetRootPath("/queue/monitoring")
	return dashboard.Handler()
}

func (q *RedisQueue) Inspector() backend.Backend {
	return q.backend
}

func (q *RedisQueue) Client() *surge.Client {
	return q.client
}

func (q *RedisQueue) DeleteEventDeliveriesFromQueue(queueName convoy.QueueName, ids []string) error {
	ctx := context.Background()

	for _, id := range ids {
		namespace := extractNamespaceFromJobID(id)
		dlqJobs, err := q.backend.InspectDLQ(ctx, namespace, string(queueName), 0, 1000)
		if err != nil {
			continue
		}

		for _, dlqJob := range dlqJobs {
			var taskPayload TaskPayload
			if err := json.Unmarshal(dlqJob.Args, &taskPayload); err == nil {
				if taskPayload.ID == id {
					if retryErr := q.backend.RetryFromDLQ(ctx, dlqJob.ID); retryErr != nil {
						continue
					}
				}
			}
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
