package worker

import (
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/worker/task"
	"github.com/go-co-op/gocron"
	log "github.com/sirupsen/logrus"
)

type Scheduler struct {
	inner   *gocron.Scheduler
	queue   queue.Queuer
	msgRepo convoy.MessageRepository
}

func NewScheduler(queue queue.Queuer, msgRepo convoy.MessageRepository) *Scheduler {
	return &Scheduler{
		inner:   gocron.NewScheduler(time.UTC),
		queue:   queue,
		msgRepo: msgRepo,
	}
}

func (s *Scheduler) Start() {
	s.addTask("post", 5, task.PostMessages)
	s.addTask("retry", 1, task.RetryMessages)

	s.inner.StartAsync()
}

func (s *Scheduler) addTask(name string, secs int, task func(queue.Queuer, convoy.MessageRepository)) {
	_, err := s.inner.Every(secs).Seconds().Do(func() {
		task(s.queue, s.msgRepo)
	})
	if err != nil {
		log.Fatalf("Failed to add %s scheduler task", name)
	}
}
