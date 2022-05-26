package task

import (
	"context"
	"fmt"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/disq"
	log "github.com/sirupsen/logrus"
)

func CreateTask(name convoy.TaskName, group datastore.Group, handler interface{}) (*disq.Task, error) {

	options := disq.TaskOptions{
		Name:       string(name),
		RetryLimit: int(group.Config.Strategy.RetryCount),
		Handler:    handler,
	}
	task, err := disq.RegisterTask(&options)
	if err != nil {
		return nil, err
	}
	return task, nil
}

func CreateTasks(groupRepo datastore.GroupRepository, taskname convoy.TaskName, handler interface{}) error {
	var name convoy.TaskName
	filter := &datastore.GroupFilter{}

	groups, err := groupRepo.LoadGroups(context.Background(), filter)
	if err != nil {
		log.WithError(err).Error("Monitor failed to load groups.")
		return err
	}

	for _, g := range groups {
		name = taskname.SetPrefix(g.Name)
		t, _ := disq.Tasks.LoadTask(string(name))
		if t == nil {
			log.Infof("Registering task handler for %s", g.Name)
			_, err := CreateTask(name, *g, handler)
			if err != nil {
				log.WithError(err).Error(fmt.Sprintf("failed register event delivery task for group %v", g.Name))
			}
		}
	}
	return nil
}
