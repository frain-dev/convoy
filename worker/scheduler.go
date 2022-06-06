package worker

import (
	"context"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/queue"
	"github.com/hibiken/asynq"
	log "github.com/sirupsen/logrus"
)

type Scheduler struct {
	queue    queue.Queuer
	inner    *asynq.Scheduler
	consumer *Consumer
}

func NewScheduler(queue queue.Queuer) *Scheduler {
	scheduler := asynq.NewScheduler(asynq.RedisClientOpt{
		Addr:     queue.Options().RedisAddress,
		Password: "",
		DB:       0,
	}, nil)

	w, err := NewConsumer(queue)
	if err != nil {
		log.WithError(err).Fatal("error creating consumer")
	}

	return &Scheduler{
		inner:    scheduler,
		queue:    queue,
		consumer: w,
	}
}

func (s *Scheduler) Start() {
	s.consumer.Start()

	if err := s.inner.Start(); err != nil {
		log.Fatal(err)
	}
}

func (s *Scheduler) RegisterTask(cronspec string, taskName convoy.TaskName, payload []byte) {
	task := asynq.NewTask(string(taskName), payload)

	_, err := s.inner.Register(cronspec, task, asynq.Queue(string(convoy.SchduleQueue)))
	if err != nil {
		log.WithError(err).Fatalf("Failed to register %s scheduler task", taskName)
	}
}

func (s *Scheduler) RegisterTaskHandler(taskName convoy.TaskName, taskHandler func(context.Context, *asynq.Task) error) {
	s.consumer.RegisterHandlers(taskName, taskHandler)
}

func (s *Scheduler) Stop() {
	s.inner.Shutdown()
}
