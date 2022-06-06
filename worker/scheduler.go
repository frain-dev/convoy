package worker

import (
	"time"

	"github.com/frain-dev/convoy/queue"
	"github.com/go-co-op/gocron"
	log "github.com/sirupsen/logrus"
)

type Scheduler struct {
	inner *gocron.Scheduler
	queue *queue.Queuer
}

func NewScheduler(queue *queue.Queuer) *Scheduler {
	return &Scheduler{
		inner: gocron.NewScheduler(time.UTC),
		queue: queue,
	}
}

func (s *Scheduler) Start() {
	s.inner.StartBlocking()
}

func (s *Scheduler) AddTask(name string, secs int, task interface{}) {
	_, err := s.inner.Every(secs).Seconds().Do(task)
	if err != nil {
		log.WithError(err).Fatalf("Failed to add %s scheduler task", name)
	}
}
