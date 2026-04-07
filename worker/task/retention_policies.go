package task

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v9"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"

	"github.com/frain-dev/convoy/datastore"
	blobstore "github.com/frain-dev/convoy/internal/pkg/blob-store"
	"github.com/frain-dev/convoy/internal/pkg/exporter"
	"github.com/frain-dev/convoy/internal/pkg/retention"
	log "github.com/frain-dev/convoy/pkg/logger"
)

func ExportTableData(configRepo datastore.ConfigurationRepository, projectRepo datastore.ProjectRepository,
	eventRepo datastore.EventRepository, eventDeliveryRepo datastore.EventDeliveryRepository, attemptsRepo datastore.DeliveryAttemptsRepository, rd redis.UniversalClient, logger log.Logger) func(context.Context, *asynq.Task) error {
	pool := goredis.NewPool(rd)
	rs := redsync.New(pool)

	return func(ctx context.Context, t *asynq.Task) error {
		const mutexName = "convoy:backup-project-data:mutex"
		mutex := rs.NewMutex(mutexName, redsync.WithExpiry(30*time.Minute), redsync.WithTries(1))

		lockCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		err := mutex.LockContext(lockCtx)
		if err != nil {
			return fmt.Errorf("failed to obtain lock: %v", err)
		}

		// Renew lock periodically to prevent expiry during long-running exports
		renewDone := make(chan struct{})
		go func() {
			ticker := time.NewTicker(10 * time.Minute)
			defer ticker.Stop()
			for {
				select {
				case <-renewDone:
					return
				case <-ticker.C:
					renewCtx, renewCancel := context.WithTimeout(ctx, 10*time.Second)
					_, _ = mutex.ExtendContext(renewCtx)
					renewCancel()
				}
			}
		}()

		defer func() {
			close(renewDone)
			_ctx, _cancel := context.WithTimeout(ctx, time.Second*2)
			defer _cancel()

			ok, _err := mutex.UnlockContext(_ctx)
			if !ok || _err != nil {
				logger.Error("failed to release lock", "error", _err)
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
			logger.Warn("no existing projects, retention policy job exiting")
			return nil
		}

		// Create blob store client once for all projects
		blobStoreClient, err := blobstore.NewBlobStoreClient(config.StoragePolicy, logger)
		if err != nil {
			return err
		}

		for _, p := range projects {
			e, innerErr := exporter.NewExporter(projectRepo, eventRepo, eventDeliveryRepo, p, config, attemptsRepo, logger)
			if innerErr != nil {
				return innerErr
			}

			// Stream export directly to blob storage (no local disk needed)
			_, innerErr = e.StreamExport(ctx, blobStoreClient)
			if innerErr != nil {
				logger.Error(fmt.Sprintf("Failed to archive project id's (%s) events : %v", p.UID, innerErr))
			}
		}

		logger.Info(fmt.Sprintf("Backup Project Data job took %f minutes to run", time.Since(c).Minutes()))
		return nil
	}
}

func RetentionPolicies(rd redis.UniversalClient, ret retention.Retentioner, logger log.Logger) func(context.Context, *asynq.Task) error {
	pool := goredis.NewPool(rd)
	rs := redsync.New(pool)

	return func(ctx context.Context, t *asynq.Task) error {
		const mutexName = "convoy:retention:mutex"
		mutex := rs.NewMutex(mutexName, redsync.WithExpiry(30*time.Minute), redsync.WithTries(1))

		lockCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		err := mutex.LockContext(lockCtx)
		if err != nil {
			return fmt.Errorf("failed to obtain lock: %v", err)
		}

		// Renew lock periodically to prevent expiry during long-running retention
		renewDone := make(chan struct{})
		go func() {
			ticker := time.NewTicker(10 * time.Minute)
			defer ticker.Stop()
			for {
				select {
				case <-renewDone:
					return
				case <-ticker.C:
					renewCtx, renewCancel := context.WithTimeout(ctx, 10*time.Second)
					_, _ = mutex.ExtendContext(renewCtx)
					renewCancel()
				}
			}
		}()

		defer func() {
			close(renewDone)
			_lockCtx, _cancel := context.WithTimeout(ctx, time.Second*2)
			defer _cancel()

			ok, _err := mutex.UnlockContext(_lockCtx)
			if !ok || _err != nil {
				logger.ErrorContext(ctx, "failed to release lock", "error", _err)
			}
		}()

		c := time.Now()
		err = ret.Perform(ctx)
		if err != nil {
			return err
		}

		logger.InfoContext(ctx, fmt.Sprintf("Retention job took %f minutes to run", time.Since(c).Minutes()))
		return nil
	}
}
