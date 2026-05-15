package task

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v9"
	"github.com/hibiken/asynq"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/organisations"
	"github.com/frain-dev/convoy/internal/pkg/billing"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
	log "github.com/frain-dev/convoy/pkg/logger"
)

const orgStatusUpdatePerPage = 50

func applySubscriptionDerivedDisabledState(
	ctx context.Context,
	orgRepo datastore.OrganisationRepository,
	orgs []datastore.Organisation,
	hasActiveSubscription bool,
	logger log.Logger,
) (updatedCount, errorCount int) {
	for i := range orgs {
		org := orgs[i]
		if hasActiveSubscription {
			if org.DisabledAt.Valid {
				org.DisabledAt = null.Time{}
				if err := orgRepo.UpdateOrganisation(ctx, &org); err != nil {
					logger.Errorf("Failed to clear organisation %s disabled_at: %v", org.UID, err)
					errorCount++
					continue
				}
				updatedCount++
				logger.Infof("Cleared organisation %s disabled_at - subscription active", org.UID)
			}
		} else {
			if !org.DisabledAt.Valid {
				org.DisabledAt = null.NewTime(time.Now(), true)
				if err := orgRepo.UpdateOrganisation(ctx, &org); err != nil {
					logger.Errorf("Failed to set organisation %s disabled_at: %v", org.UID, err)
					errorCount++
					continue
				}
				updatedCount++
				logger.Infof("Set organisation %s disabled_at - subscription not active", org.UID)
			}
		}
	}
	return updatedCount, errorCount
}

func UpdateOrganisationStatus(db database.Database, billingClient billing.Client, rd *rdb.Redis, logger log.Logger) func(context.Context, *asynq.Task) error {
	pool := goredis.NewPool(rd.Client())
	rs := redsync.New(pool)

	return func(ctx context.Context, t *asynq.Task) error {
		if billingClient == nil {
			logger.Info("Billing client not configured, skipping organisation status update")
			return nil
		}

		cfg, err := config.Get()
		if err != nil {
			return fmt.Errorf("failed to get config: %w", err)
		}

		const mutexName = "convoy:update_organisation_status:mutex"
		mutex := rs.NewMutex(mutexName, redsync.WithExpiry(time.Minute*30), redsync.WithTries(1))

		tctx, cancel := context.WithTimeout(ctx, time.Second*2)
		defer cancel()

		err = mutex.LockContext(tctx)
		if err != nil {
			return fmt.Errorf("failed to obtain lock: %v", err)
		}

		defer func() {
			tctx, cancel := context.WithTimeout(ctx, time.Second*2)
			defer cancel()

			ok, err := mutex.UnlockContext(tctx)
			if !ok || err != nil {
				logger.Error("failed to release lock", "error", err)
			}
		}()

		orgRepo := organisations.New(logger, db)
		orgs, err := getAllOrganisationsForStatusUpdate(ctx, orgRepo)
		if err != nil {
			return fmt.Errorf("failed to fetch organisations: %w", err)
		}

		logger.Infof("Updating status for %d organisations", len(orgs))

		updatedCount := 0
		errorCount := 0

		if !cfg.IsCloud() {
			logger.Info("Organisation status update: skipping disabled_at updates outside cloud mode")
			return nil
		}

		for _, org := range orgs {
			resp, err := billingClient.GetSubscription(ctx, org.UID)
			if err != nil {
				logger.Errorf("Failed to fetch subscription for organisation %s: %v", org.UID, err)
				errorCount++
				continue
			}

			hasActiveSubscription := billing.HasActiveSubscription(resp.Data)
			u, e := applySubscriptionDerivedDisabledState(ctx, orgRepo, []datastore.Organisation{org}, hasActiveSubscription, logger)
			updatedCount += u
			errorCount += e
		}

		logger.Infof("Organisation status update completed: %d updated, %d errors", updatedCount, errorCount)
		return nil
	}
}

func getAllOrganisationsForStatusUpdate(ctx context.Context, orgRepo datastore.OrganisationRepository) ([]datastore.Organisation, error) {
	var cursor = "FFFFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF"
	var orgs []datastore.Organisation

	for {
		paged, pagination, err := orgRepo.LoadOrganisationsPaged(ctx, datastore.Pageable{PerPage: orgStatusUpdatePerPage, NextCursor: cursor, Direction: datastore.Next})
		if err != nil {
			return nil, err
		}

		orgs = append(orgs, paged...)

		if len(paged) == 0 && !pagination.HasNextPage {
			break
		}

		cursor = pagination.NextPageCursor
	}

	return orgs, nil
}
