package task

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/hibiken/asynq"
	"time"
)

type SearchIndexParams struct {
	ProjectID string `json:"project_id"`
	Interval  int    `json:"interval"`
}

func GeneralTokenizerHandler(projectRepository datastore.ProjectRepository, eventRepo datastore.EventRepository) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		projectEvents, err := projectRepository.GetProjectsWithEventsInTheInterval(ctx, 0)
		if err != nil {
			return err
		}

		for _, p := range projectEvents {
			err = Tokenize(ctx, eventRepo, p.Id, 100)
			if err != nil {
				log.WithError(err).Errorf("failed to tokenize events for project with id %s", p.Id)
				continue
			}
			fmt.Printf("done tokenizing events for %+v with %v events\n", p.Id, p.EventsCount)
		}
		fmt.Println("done tokenizing events in the interval")

		return nil
	}
}

func TokenizerHandler(eventRepo datastore.EventRepository) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		var data SearchIndexParams
		err := json.Unmarshal(t.Payload(), &data)
		if err != nil {
			log.WithError(err).Error("failed to unmarshal tokenizer handler payload")
			return &EndpointError{Err: err, delay: time.Second * 30}
		}

		err = Tokenize(ctx, eventRepo, data.ProjectID, data.Interval)
		if err != nil {
			return err
		}

		return nil
	}
}

func Tokenize(ctx context.Context, eventRepo datastore.EventRepository, projectId string, interval int) error {
	// check if a job for a given project is currently running

	// if a job is in progress, exit

	// if a job is not currently running start a new job
	return eventRepo.CopyRows(ctx, projectId, interval)

	// if the function returned an error, make the job as complete and failed

	// if the rows were copied without an error, mark the job as complete and successful

	// exit
}
