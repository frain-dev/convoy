package task

import (
	"context"
	"errors"
	"fmt"
	partman "github.com/jirevwe/go_partman"
	"time"

	"github.com/frain-dev/convoy/internal/pkg/rdb"
	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v9"

	"github.com/frain-dev/convoy/internal/pkg/exporter"

	"github.com/frain-dev/convoy/datastore"
	objectstore "github.com/frain-dev/convoy/internal/pkg/object-store"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/hibiken/asynq"
)

func BackupProjectData(configRepo datastore.ConfigurationRepository, projectRepo datastore.ProjectRepository, eventRepo datastore.EventRepository, eventDeliveryRepo datastore.EventDeliveryRepository, attemptsRepo datastore.DeliveryAttemptsRepository, rd *rdb.Redis) func(context.Context, *asynq.Task) error {
	pool := goredis.NewPool(rd.Client())
	rs := redsync.New(pool)

	return func(ctx context.Context, t *asynq.Task) error {
		const mutexName = "convoy:backup-project-data:mutex"
		mutex := rs.NewMutex(mutexName, redsync.WithExpiry(time.Second), redsync.WithTries(1))

		ctx, cancel := context.WithTimeout(ctx, time.Second*2)
		defer cancel()

		err := mutex.LockContext(ctx)
		if err != nil {
			return fmt.Errorf("failed to obtain lock: %v", err)
		}

		defer func() {
			_ctx, _cancel := context.WithTimeout(ctx, time.Second*2)
			defer _cancel()

			ok, _err := mutex.UnlockContext(_ctx)
			if !ok || _err != nil {
				log.WithError(_err).Error("failed to release lock")
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
			log.Warn("no existing projects, retention policy job exiting")
			return nil
		}

		for _, p := range projects {
			e, innerErr := exporter.NewExporter(projectRepo, eventRepo, eventDeliveryRepo, p, config, attemptsRepo)
			if innerErr != nil {
				return innerErr
			}

			result, innerErr := e.Export(ctx)
			if innerErr != nil {
				log.WithError(innerErr).Errorf("Failed to archive project id's (%s) events ", p.UID)
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

		log.Printf("Backup job took %f minutes to run", time.Since(c).Minutes())
		return nil
	}
}

func RetentionPolicies(rd *rdb.Redis, manager *partman.Manager) func(context.Context, *asynq.Task) error {
	pool := goredis.NewPool(rd.Client())
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
				log.FromContext(ctx).WithError(_err).Error("failed to release lock")
			}
		}()

		c := time.Now()
		err = manager.Maintain(ctx)
		if err != nil {
			return err
		}

		log.FromContext(ctx).Infof("Backup job took %f minutes to run", time.Since(c).Minutes())
		return nil
	}
}
