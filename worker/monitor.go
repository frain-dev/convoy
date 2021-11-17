package worker

import (
	"context"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/worker/task"
	log "github.com/sirupsen/logrus"
	"github.com/vmihailenco/taskq/v3"
)

type Monitor struct {
	handler   func(*queue.Job) error
	groupRepo convoy.GroupRepository
	quit      chan chan error
}

func NewMonitor(handler func(*queue.Job) error, g convoy.GroupRepository) *Monitor {
	return &Monitor{
		handler:   handler,
		groupRepo: g,
	}
}

func (m *Monitor) Start() {
	go func() {
		var name convoy.TaskName
		filter := &convoy.GroupFilter{}
		for {

			groups, err := m.groupRepo.LoadGroups(context.Background(), filter)
			if err != nil {
				log.WithError(err).Error("Monitor failed to load groups.")
			}

			for _, g := range groups {
				name = convoy.EventProcessor.SetPrefix(g.Name)

				if t := taskq.Tasks.Get(string(name)); t == nil {
					log.Infof("Registering task handler for %s", g.Name)
					task.CreateTask(name, *g, m.handler)
				}
			}

			// sleep for 10 seconds.
			time.Sleep(10 * time.Second)
		}
	}()
}

func (m *Monitor) Close() error {
	ch := make(chan error)
	m.quit <- ch
	return <-ch
}
