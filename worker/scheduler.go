package worker

import (
	"context"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/limiter"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/worker/task"
	"github.com/frain-dev/disq"
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
		log.WithError(err).Errorf("Failed to add %s scheduler task", name)
	}
}

func RegisterNewGroupTask(applicationRepo datastore.ApplicationRepository, eventDeliveryRepo datastore.EventDeliveryRepository, groupRepo datastore.GroupRepository, rateLimiter limiter.RateLimiter, eventRepo datastore.EventRepository, cache cache.Cache, eventQueue queue.Queuer) {
	go func() {
		for {
			filter := &datastore.GroupFilter{}
			groups, err := groupRepo.LoadGroups(context.Background(), filter)
			if err != nil {
				log.WithError(err).Error("failed to load groups")
			}
			for _, g := range groups {
				pEvtDelTask := convoy.EventProcessor.SetPrefix(g.Name)       // process event delivery task
				pEvtCrtTask := convoy.CreateEventProcessor.SetPrefix(g.Name) // process event create task

				t, _ := disq.Tasks.LoadTask(string(pEvtCrtTask))
				if t == nil {
					s, _ := disq.Tasks.LoadTask(string(pEvtDelTask))
					if s == nil {
						handler := task.ProcessEventDelivery(applicationRepo, eventDeliveryRepo, groupRepo, rateLimiter)
						log.Infof("Registering event delivery task handler for %s", g.Name)
						task.CreateTask(pEvtDelTask, *g, handler)

						eventCreatedhandler := task.ProcessEventCreated(applicationRepo, eventRepo, groupRepo, eventDeliveryRepo, cache, eventQueue)
						log.Infof("Registering event creation task handler for %s", g.Name)
						task.CreateTask(pEvtCrtTask, *g, eventCreatedhandler)
					}
				}
			}
		}
	}()
}
