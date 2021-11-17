package task

import (
	"context"

	"github.com/frain-dev/convoy"
	log "github.com/sirupsen/logrus"
	"github.com/vmihailenco/taskq/v3"
)

func CreateTask(name convoy.TaskName, group convoy.Group, handler interface{}) *taskq.Task {

	options := taskq.TaskOptions{
		Name:       string(name),
		RetryLimit: int(group.Config.Strategy.Default.RetryLimit),
		Handler:    handler,
	}

	return taskq.RegisterTask(&options)
}

func CreateTasks(groupRepo convoy.GroupRepository, handler interface{}) error {
	var name convoy.TaskName
	filter := &convoy.GroupFilter{}

	groups, err := groupRepo.LoadGroups(context.Background(), filter)
	if err != nil {
		log.WithError(err).Error("Monitor failed to load groups.")
		return err
	}

	for _, g := range groups {
		name = convoy.EventProcessor.SetPrefix(g.Name)

		if t := taskq.Tasks.Get(string(name)); t == nil {
			log.Infof("Registering task handler for %s", g.Name)
			CreateTask(name, *g, handler)
		}
	}

	return nil
}
