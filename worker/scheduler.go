package worker

import (
	"github.com/go-co-op/gocron"
	"github.com/hookcamp/hookcamp"
	"github.com/hookcamp/hookcamp/queue"
	"github.com/hookcamp/hookcamp/worker/task"
	log "github.com/sirupsen/logrus"
	"time"
)

type Scheduler struct {
	inner   *gocron.Scheduler
	queue   *queue.Queuer
	msgRepo *hookcamp.MessageRepository
}

func NewScheduler(queue *queue.Queuer, msgRepo *hookcamp.MessageRepository) *Scheduler {
	return &Scheduler{
		inner:   gocron.NewScheduler(time.UTC),
		queue:   queue,
		msgRepo: msgRepo,
	}
}

func (s *Scheduler) Start() {
	s.addTask("post", 5, task.PostMessages)
	s.addTask("retry", 15, task.RetryMessages)

	s.inner.StartAsync()
}

func (s *Scheduler) addTask(name string, secs int, task func(queue.Queuer, hookcamp.MessageRepository)) {
	_, err := s.inner.Every(secs).Seconds().Do(func() {
		task(*s.queue, *s.msgRepo)
	})
	if err != nil {
		log.Fatalf("Failed to add %s scheduler task", name)
	}
}
