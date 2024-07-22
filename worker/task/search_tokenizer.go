package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	fflag2 "github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v9"
	"github.com/hibiken/asynq"
	"github.com/oklog/ulid/v2"
	"time"
)

func GeneralTokenizerHandler(projectRepository datastore.ProjectRepository, eventRepo datastore.EventRepository, jobRepo datastore.JobRepository, redis *rdb.Redis) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		pool := goredis.NewPool(redis.Client())
		rs := redsync.New(pool)

		const mutexName = "convoy:general_tokenizer:mutex"
		mutex := rs.NewMutex(mutexName, redsync.WithExpiry(time.Second), redsync.WithTries(1))

		tctx, cancel := context.WithTimeout(ctx, time.Second*2)
		defer cancel()

		err := mutex.LockContext(tctx)
		if err != nil {
			return fmt.Errorf("failed to obtain lock: %v", err)
		}

		defer func() {
			tctx, cancel := context.WithTimeout(ctx, time.Second*2)
			defer cancel()

			ok, err := mutex.UnlockContext(tctx)
			if !ok || err != nil {
				log.WithError(err).Error("failed to release lock")
			}
		}()

		projectEvents, err := projectRepository.GetProjectsWithEventsInTheInterval(ctx, config.DefaultSearchTokenizationInterval)
		if err != nil {
			return err
		}

		for _, p := range projectEvents {
			err = tokenize(ctx, eventRepo, jobRepo, p.Id, config.DefaultSearchTokenizationInterval)
			if err != nil {
				log.WithError(err).Errorf("failed to tokenize events for project with id %s", p.Id)
				continue
			}
			log.Debugf("done tokenizing events for %+v with %v events", p.Id, p.EventsCount)
		}
		log.Debugf("done tokenizing events in the interval")

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

		err = tokenize(ctx, eventRepo, jobRepo, params.ProjectID, params.Interval)
		if err != nil {
			return err
		}
		log.Debugf("done tokenizing events in the last %d hours for project with id %s", params.Interval, params.ProjectID)

		return nil
	}
}

func tokenize(ctx context.Context, eventRepo datastore.EventRepository, jobRepo datastore.JobRepository, projectId string, interval int) error {
	cfg, err := config.Get()
	if err != nil {
		return err
	}
	fflag, err := fflag2.NewFFlag(&cfg)
	if err != nil {
		return nil
	}
	if !fflag.CanAccessFeature(fflag2.Search) {
		return fflag2.ErrFeatureNotEnabled
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
