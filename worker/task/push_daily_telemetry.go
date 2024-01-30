package task

import (
	"context"
	"fmt"
	"time"

	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
	"github.com/frain-dev/convoy/internal/telemetry"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v9"
	"github.com/hibiken/asynq"
)

func PushDailyTelemetry(log *log.Logger, db database.Database, cache cache.Cache, cfg config.Configuration, rd *rdb.Redis) func(context.Context, *asynq.Task) error {
	// Create a pool with go-redis
	pool := goredis.NewPool(rd.Client())
	rs := redsync.New(pool)

	// Do your work that requires the lock.

	return func(ctx context.Context, t *asynq.Task) error {
		// Obtain a new mutex by using the same name for all instances wanting the
		// same lock.
		const mutexName = "convoy:analytics:mutex"
		mutex := rs.NewMutex(mutexName, redsync.WithExpiry(time.Second), redsync.WithTries(1))

		tctx, cancel := context.WithTimeout(ctx, time.Second*2)
		defer cancel()

		// Obtain a lock for our given mutex. After this is successful, no one else
		// can obtain the same lock (the same mutex name) until we unlock it.
		err := mutex.LockContext(tctx)
		if err != nil {
			return fmt.Errorf("failed to obtain lock: %v", err)
		}

		defer func() {
			tctx, cancel := context.WithTimeout(ctx, time.Second*2)
			defer cancel()

			// Release the lock so other processes or threads can obtain a lock.
			ok, err := mutex.UnlockContext(tctx)
			if !ok || err != nil {
				log.WithError(err).Error("failed to release lock")
			}
		}()

		configRepo := postgres.NewConfigRepo(db)
		eventRepo := postgres.NewEventRepo(db, cache)
		projectRepo := postgres.NewProjectRepo(db, cache)
		orgRepo := postgres.NewOrgRepo(db, cache)

		totalEventsTracker := &telemetry.TotalEventsTracker{
			OrgRepo:     orgRepo,
			EventRepo:   eventRepo,
			ConfigRepo:  configRepo,
			ProjectRepo: projectRepo,
		}

		totalActiveProjectTracker := &telemetry.TotalActiveProjectTracker{
			OrgRepo:     orgRepo,
			EventRepo:   eventRepo,
			ConfigRepo:  configRepo,
			ProjectRepo: projectRepo,
		}

		telemetry := telemetry.NewTelemetry(log,
			telemetry.OptionTracker(totalEventsTracker),
			telemetry.OptionTracker(totalActiveProjectTracker))

		err = telemetry.Capture(ctx)
		if err != nil {
			return err
		}

		return nil
	}
}
