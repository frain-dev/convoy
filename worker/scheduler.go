package worker

import (
	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	"github.com/hibiken/asynq"
)

type Scheduler struct {
	log   log.StdLogger
	queue queue.Queuer
	inner *asynq.Scheduler
}

func NewScheduler(queue queue.Queuer, log log.StdLogger) *Scheduler {
	scheduler := asynq.NewScheduler(queue.Options().RedisClient, &asynq.SchedulerOpts{
		Logger: log,
	})

	return &Scheduler{
		log:   log,
		inner: scheduler,
		queue: queue,
	}
}

func (s *Scheduler) Start() {
	if err := s.inner.Start(); err != nil {
		s.log.WithError(err).Fatal("Could not start scheduler")
	}
}

func (s *Scheduler) RegisterTask(cronspec string, queue convoy.QueueName, taskName convoy.TaskName) {
	task := asynq.NewTask(string(taskName), nil)
	_, err := s.inner.Register(cronspec, task, asynq.Queue(string(queue)))
	if err != nil {
		s.log.WithError(err).Fatalf("Failed to register %s scheduler task", taskName)
	}
}

func (s *Scheduler) Stop() {
	s.inner.Shutdown()
}
