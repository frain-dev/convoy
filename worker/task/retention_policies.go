package task

import (
	"context"
	"errors"
	"fmt"
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

func RetentionPolicies(configRepo datastore.ConfigurationRepository, projectRepo datastore.ProjectRepository, eventRepo datastore.EventRepository, eventDeliveryRepo datastore.EventDeliveryRepository, attemptsRepo datastore.DeliveryAttemptsRepository, rd *rdb.Redis) func(context.Context, *asynq.Task) error {
	pool := goredis.NewPool(rd.Client())
	rs := redsync.New(pool)

	return func(ctx context.Context, t *asynq.Task) error {
		const mutexName = "convoy:retention:mutex"
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
			log.Infof("no existing projects, retention policy job exiting")
			return nil
		}

		for _, p := range projects {
			e, err := exporter.NewExporter(projectRepo, eventRepo, eventDeliveryRepo, p, config, attemptsRepo)
			if err != nil {
				return err
			}

			result, err := e.Export(ctx)
			if err != nil {
				log.WithError(err).Errorf("Failed to archive project id's (%s) events ", p.UID)
			}

			// upload to object storage.
			objectStoreClient, err := objectstore.NewObjectStoreClient(config.StoragePolicy)
			if err != nil {
				return err
			}

			for _, r := range result {
				if r.NumDocs > 0 { // skip if no record was exported
					err = objectStoreClient.Save(r.ExportFile)
					if err != nil {
						return err
					}
				}
			}

			// prune tables and files.
			err = e.Cleanup(ctx)
			if err != nil {
				return err
			}
		}

		log.Printf("Retention policy job took %f minutes to run", time.Since(c).Minutes())
		return nil
	}
}
