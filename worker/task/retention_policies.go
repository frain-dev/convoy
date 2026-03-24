package task

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v9"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/exporter"
	objectstore "github.com/frain-dev/convoy/internal/pkg/object-store"
	"github.com/frain-dev/convoy/internal/pkg/retention"
)

func BackupProjectData(configRepo datastore.ConfigurationRepository, projectRepo datastore.ProjectRepository,
	eventRepo datastore.EventRepository, eventDeliveryRepo datastore.EventDeliveryRepository, attemptsRepo datastore.DeliveryAttemptsRepository, rd redis.UniversalClient) func(context.Context, *asynq.Task) error {
	pool := goredis.NewPool(rd)
	rs := redsync.New(pool)

	return func(ctx context.Context, t *asynq.Task) error {
		const mutexName = "convoy:backup-project-data:mutex"
		mutex := rs.NewMutex(mutexName, redsync.WithExpiry(time.Second), redsync.WithTries(1))

		innerCtx, cancel := context.WithTimeout(ctx, time.Second*2)
		defer cancel()

		err := mutex.LockContext(innerCtx)
		if err != nil {
			return fmt.Errorf("failed to obtain lock: %v", err)
		}

		defer func() {
			_ctx, _cancel := context.WithTimeout(ctx, time.Second*2)
			defer _cancel()

			ok, _err := mutex.UnlockContext(_ctx)
			if !ok || _err != nil {
				slog.Error("failed to release lock", "error", _err)
			}
		}()

		c := time.Now()
		config, err := configRepo.LoadConfiguration(ctx)
		if err != nil {
			if errors.Is(err, datastore.ErrConfigNotFound) {
				return nil
			}
			return err
		}

		filter := &datastore.ProjectFilter{}
		projects, err := projectRepo.LoadProjects(context.Background(), filter)
		if err != nil {
			return err
		}

		if len(projects) == 0 {
			slog.Warn("no existing projects, retention policy job exiting")
			return nil
		}

		for _, p := range projects {
			e, innerErr := exporter.NewExporter(projectRepo, eventRepo, eventDeliveryRepo, p, config, attemptsRepo)
			if innerErr != nil {
				return innerErr
			}

			result, innerErr := e.Export(ctx)
			if innerErr != nil {
				slog.Error(fmt.Sprintf("Failed to archive project id's (%s) events : %v", p.UID, innerErr))
			}

			// upload to object storage.
			objectStoreClient, innerErr := objectstore.NewObjectStoreClient(config.StoragePolicy)
			if innerErr != nil {
				return innerErr
			}

			for _, r := range result {
				if r.NumDocs > 0 { // skip if no record was exported
					innerErr = objectStoreClient.Save(r.ExportFile)
					if innerErr != nil {
						return innerErr
					}
				}
			}
		}

		slog.Info(fmt.Sprintf("Backup Project Data job took %f minutes to run", time.Since(c).Minutes()))
		return nil
	}
}

func RetentionPolicies(rd redis.UniversalClient, ret retention.Retentioner) func(context.Context, *asynq.Task) error {
	pool := goredis.NewPool(rd)
	rs := redsync.New(pool)

	return func(ctx context.Context, t *asynq.Task) error {
		const mutexName = "convoy:retention:mutex"
		mutex := rs.NewMutex(mutexName, redsync.WithExpiry(time.Second), redsync.WithTries(1))

		lockCtx, cancel := context.WithTimeout(ctx, time.Second*2)
		defer cancel()

		err := mutex.LockContext(lockCtx)
		if err != nil {
			return fmt.Errorf("failed to obtain lock: %v", err)
		}

		defer func() {
			_lockCtx, _cancel := context.WithTimeout(ctx, time.Second*2)
			defer _cancel()

			ok, _err := mutex.UnlockContext(_lockCtx)
			if !ok || _err != nil {
				slog.ErrorContext(ctx, "failed to release lock", "error", _err)
			}
		}()

		c := time.Now()
		err = ret.Perform(ctx)
		if err != nil {
			return err
		}

		slog.InfoContext(ctx, fmt.Sprintf("Retention job took %f minutes to run", time.Since(c).Minutes()))
		return nil
	}
}
