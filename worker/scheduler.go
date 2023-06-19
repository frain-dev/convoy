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

func (s *Scheduler) RegisterTask(cronSpec string, queue convoy.QueueName, taskName convoy.TaskName) {
	task := asynq.NewTask(string(taskName), nil)
	id, err := s.inner.Register(cronSpec, task, asynq.Queue(string(queue)))
	if err != nil {
		s.log.WithError(err).Fatalf("Failed to register %s scheduler task", taskName)
	}
	s.log.Infof("Registered task %v with id %v", taskName, id)
}

func (s *Scheduler) Stop() {
	s.inner.Shutdown()
}
