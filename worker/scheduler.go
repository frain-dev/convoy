package worker

import (
	"context"

	"github.com/oklog/ulid/v2"
	"github.com/olamilekan000/surge/surge"
	"github.com/robfig/cron/v3"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/queue/redis"
)

type Scheduler struct {
	log    log.StdLogger
	queue  queue.Queuer
	cron   *cron.Cron
	client *surge.Client
	ctx    context.Context
	cancel context.CancelFunc
}

func NewScheduler(queue queue.Queuer, log log.StdLogger) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())

	redisQueue, ok := queue.(*redis.RedisQueue)
	if !ok {
		log.Fatal("queue must be a RedisQueue for surge")
	}

	client := redisQueue.Client()
	c := cron.New()

	return &Scheduler{
		log:    log,
		queue:  queue,
		cron:   c,
		client: client,
		ctx:    ctx,
		cancel: cancel,
	}
}

func (s *Scheduler) Start() {
	s.cron.Start()
	s.log.Info("Scheduler started")
}

func (s *Scheduler) RegisterTask(cronSpec string, queueName convoy.QueueName, taskName convoy.TaskName) {
	_, err := s.cron.AddFunc(cronSpec, func() {
		jobID := ulid.Make().String()
		payload := redis.TaskPayload{
			TaskName: string(taskName),
			Payload:  []byte{},
			ID:       jobID,
			Queue:    string(queueName),
		}

		err := s.client.Job(payload).Ns("system").Enqueue(s.ctx)
		if err != nil {
			s.log.WithError(err).Errorf("Failed to enqueue scheduled task %s", taskName)
		} else {
			s.log.Infof("Enqueued scheduled task %s to queue %s", taskName, queueName)
		}
	})

	if err != nil {
		s.log.WithError(err).Fatalf("Failed to register %s scheduler task", taskName)
	}

	s.log.Infof("Registered task %v with cron spec %v", taskName, cronSpec)
}

func (s *Scheduler) Stop() {
	s.cancel()
	s.cron.Stop()
	s.log.Info("Scheduler stopped")
}
