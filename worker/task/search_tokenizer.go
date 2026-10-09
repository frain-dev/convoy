package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	fflag2 "github.com/frain-dev/convoy/internal/pkg/fflag"
	log "github.com/frain-dev/convoy/pkg/logger"
)

func GeneralTokenizerHandler(projectRepository datastore.ProjectRepository, eventRepo datastore.EventRepository, jobRepo datastore.JobRepository, locker JobLocker, logger log.Logger) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		// Copies an interval of events into the search table for every project
		// with activity, one project at a time; 30m matches the other
		// full-table maintenance jobs.
		return locker.WithLock(ctx, "convoy:general_tokenizer:mutex", 30*time.Minute, func(ctx context.Context) error {
			projectEvents, err := projectRepository.GetProjectsWithEventsInTheInterval(ctx, config.DefaultSearchTokenizationInterval)
			if err != nil {
				return err
			}

			for _, p := range projectEvents {
				err = tokenize(ctx, eventRepo, jobRepo, p.Id, config.DefaultSearchTokenizationInterval)
				if err != nil {
					logger.Error(fmt.Sprintf("failed to tokenize events for project with id %s: %v", p.Id, err))
					continue
				}
				logger.Debug(fmt.Sprintf("done tokenizing events for %+v with %v events", p.Id, p.EventsCount))
			}
			logger.Debug("done tokenizing events in the interval")

			return nil
		})
	}
}

func TokenizerHandler(eventRepo datastore.EventRepository, jobRepo datastore.JobRepository, logger log.Logger) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		var params datastore.SearchIndexParams
		err := json.Unmarshal(t.Payload(), &params)
		if err != nil {
			logger.Error("failed to unmarshal tokenizer handler payload", "error", err)
			return &EndpointError{Err: err, delay: time.Second * 30}
		}

		err = tokenize(ctx, eventRepo, jobRepo, params.ProjectID, params.Interval)
		if err != nil {
			return err
		}
		logger.Debug(fmt.Sprintf("done tokenizing events in the last %d hours for project with id %s", params.Interval, params.ProjectID))

		return nil
	}
}

func tokenize(ctx context.Context, eventRepo datastore.EventRepository, jobRepo datastore.JobRepository, projectId string, interval int) error {
	cfg, err := config.Get()
	if err != nil {
		return err
	}

	fflag := fflag2.NewFFlag(cfg.EnableFeatureFlag)

	if !fflag.CanAccessFeature(fflag2.FullTextSearch) {
		return fflag2.ErrFullTextSearchNotEnabled
	}

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

	// if a job is not currently running, start a new job
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
