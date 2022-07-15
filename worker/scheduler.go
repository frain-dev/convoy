package worker

import (
	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/queue"
	"github.com/hibiken/asynq"
	log "github.com/sirupsen/logrus"
)

type Scheduler struct {
	queue queue.Queuer
	inner *asynq.Scheduler
}

func NewScheduler(queue queue.Queuer) *Scheduler {
	scheduler := asynq.NewScheduler(queue.Options().RedisClient, nil)

	return &Scheduler{
		inner: scheduler,
		queue: queue,
	}
}

func (s *Scheduler) Start() {
	if err := s.inner.Start(); err != nil {
		log.Fatal(err)
	}
}

func (s *Scheduler) RegisterTask(cronspec string, queue convoy.QueueName, taskName convoy.TaskName) {
	task := asynq.NewTask(string(taskName), nil)
	_, err := s.inner.Register(cronspec, task, asynq.Queue(string(queue)))
	if err != nil {
		log.WithError(err).Fatalf("Failed to register %s scheduler task", taskName)
	}
}

func (s *Scheduler) Stop() {
	s.inner.Shutdown()
}
