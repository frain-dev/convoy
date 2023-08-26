package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/hibiken/asynq"
	"github.com/oklog/ulid/v2"
	"time"
)

func GeneralTokenizerHandler(projectRepository datastore.ProjectRepository, eventRepo datastore.EventRepository, jobRepo datastore.JobRepository) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		projectEvents, err := projectRepository.GetProjectsWithEventsInTheInterval(ctx, 0)
		if err != nil {
			return err
		}

		for _, p := range projectEvents {
			err = tokenize(ctx, eventRepo, jobRepo, p.Id, 100)
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

func TokenizerHandler(eventRepo datastore.EventRepository, jobRepo datastore.JobRepository) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		var params datastore.SearchIndexParams
		err := json.Unmarshal(t.Payload(), &params)
		if err != nil {
			log.WithError(err).Error("failed to unmarshal tokenizer handler payload")
			return &EndpointError{Err: err, delay: time.Second * 30}
		}

		fmt.Printf("params: %+v\n", params)

		err = tokenize(ctx, eventRepo, jobRepo, params.ProjectID, params.Interval)
		if err != nil {
			return err
		}

		return nil
	}
}

func tokenize(ctx context.Context, eventRepo datastore.EventRepository, jobRepo datastore.JobRepository, projectId string, interval int) error {
	// check if a job for a given project is currently running
	jobs, err := jobRepo.FetchRunningJobsByProjectId(ctx, projectId)
	if err != nil {
		return err
	}

	// if a job is in progress, exit
	if len(jobs) > 0 {
		return errors.New("there are currently running jobs")
	}

	job := &datastore.Job{
		UID:       ulid.Make().String(),
		Type:      "search_tokenizer",
		Status:    "ready",
		ProjectID: projectId,
	}

	err = jobRepo.CreateJob(ctx, job)
	if err != nil {
		return err
	}

	err = jobRepo.MarkJobAsStarted(ctx, job.UID, projectId)
	if err != nil {
		return err
	}

	// if a job is not currently running start a new job
	err = eventRepo.CopyRows(ctx, projectId, interval)
	if err != nil {
		err = jobRepo.MarkJobAsFailed(ctx, job.UID, projectId)
		if err != nil {
			return err
		}

		return err
	}

	// if the rows were copied without an error, mark the job as complete and successful
	err = jobRepo.MarkJobAsCompleted(ctx, job.UID, projectId)
	if err != nil {
		return err
	}

	// exit
	return nil
}
