package worker

import (
	"context"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/worker/task"
	log "github.com/sirupsen/logrus"
	"github.com/vmihailenco/taskq/v3"
)

func NewGroupTask(groupRepo datastore.GroupRepository, applicationRepo datastore.ApplicationRepository, eventDeliveryRepo datastore.EventDeliveryRepository) {
	go func() {
		for {
			filter := &datastore.GroupFilter{}
			groups, err := groupRepo.LoadGroups(context.Background(), filter)
			if err != nil {
				log.Fatalf("an error occurred while fetching Groups:%v", err)
			}
			for _, g := range groups {
				name := convoy.TaskName(g.Name)
				if t := taskq.Tasks.Get(string(name)); t == nil {
					handler := task.ProcessEventDelivery(applicationRepo, eventDeliveryRepo, groupRepo)
					log.Infof("Registering task handler for %s", g.Name)
					task.CreateTask(name, *g, handler)
				}
			}
		}
	}()
}
