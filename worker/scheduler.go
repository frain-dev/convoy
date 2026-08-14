package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/robfig/cron/v3"

	"github.com/frain-dev/convoy"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/queue"
)

type Scheduler interface {
	Start()
	RegisterTask(cronSpec string, queueName convoy.QueueName, taskName convoy.TaskName)
	Stop()
}

type redisScheduler struct {
	log   log.Logger
	inner *asynq.Scheduler
}

type postgresScheduler struct {
	log   log.Logger
	queue queue.Queuer
	cron  *cron.Cron
}

func NewRedisScheduler(opts queue.QueueOptions, logger log.Logger) (Scheduler, error) {
	var redisConnOpt asynq.RedisConnOpt
	if opts.RedisFailoverOpt != nil {
		redisConnOpt = *opts.RedisFailoverOpt
	} else if opts.RedisClient != nil {
		redisConnOpt = opts.RedisClient
	} else {
		return nil, fmt.Errorf("redis scheduler connection is required")
	}
	scheduler := asynq.NewScheduler(redisConnOpt, &asynq.SchedulerOpts{
		Logger: logger,
	})

	return &redisScheduler{
		log:   logger,
		inner: scheduler,
	}, nil
}

func NewPostgresScheduler(q queue.Queuer, logger log.Logger) Scheduler {
	return &postgresScheduler{log: logger, queue: q, cron: cron.New()}
}

func (s *redisScheduler) Start() {
	if err := s.inner.Start(); err != nil {
		s.log.Fatal("Could not start scheduler", "error", err)
	}
}

func (s *redisScheduler) RegisterTask(cronSpec string, queueName convoy.QueueName, taskName convoy.TaskName) {
	task := asynq.NewTask(string(taskName), nil)
	id, err := s.inner.Register(cronSpec, task, asynq.Queue(string(queueName)))
	if err != nil {
		s.log.Fatalf("Failed to register %s scheduler task: %v", taskName, err)
	}
	s.log.Infof("Registered task %v with id %v", taskName, id)
}

func (s *redisScheduler) Stop() {
	s.inner.Shutdown()
}

func (s *postgresScheduler) Start() {
	s.cron.Start()
}

func (s *postgresScheduler) RegisterTask(cronSpec string, queueName convoy.QueueName, taskName convoy.TaskName) {
	id, err := s.cron.AddFunc(cronSpec, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// The minute-scoped ID makes cron enqueue idempotent across server
		// replicas while allowing the next scheduled tick to run.
		jobID := fmt.Sprintf("cron:%s:%d", taskName, time.Now().UTC().Truncate(time.Minute).Unix())
		if err := s.queue.Write(ctx, taskName, queueName, &queue.Job{ID: jobID}); err != nil {
			s.log.Error("postgres scheduler enqueue failed", "task", taskName, "error", err)
		}
	})
	if err != nil {
		s.log.Fatalf("Failed to register %s scheduler task: %v", taskName, err)
	}
	s.log.Infof("Registered task %v with id %v", taskName, id)
}

func (s *postgresScheduler) Stop() {
	<-s.cron.Stop().Done()
}
