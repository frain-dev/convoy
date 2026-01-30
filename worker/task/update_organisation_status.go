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
	"github.com/frain-dev/convoy/pkg/log"
)

const orgStatusUpdatePerPage = 50

func UpdateOrganisationStatus(db database.Database, billingClient billing.Client, rd *rdb.Redis, logger log.StdLogger) func(context.Context, *asynq.Task) error {
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

		if !cfg.Billing.Enabled {
			logger.Info("Billing is not enabled, skipping organisation status update")
			return nil
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
				logger.WithError(err).Error("failed to release lock")
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

		for _, org := range orgs {
			resp, err := billingClient.GetSubscription(ctx, org.UID)
			if err != nil {
				logger.WithError(err).Errorf("Failed to fetch subscription for organisation %s", org.UID)
				errorCount++
				continue
			}

			hasActiveSubscription := billing.HasActiveSubscription(resp.Data)

			if hasActiveSubscription {
				if org.DisabledAt.Valid {
					org.DisabledAt = null.Time{}
					if err := orgRepo.UpdateOrganisation(ctx, &org); err != nil {
						logger.WithError(err).Errorf("Failed to clear organisation %s disabled_at", org.UID)
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
						logger.WithError(err).Errorf("Failed to set organisation %s disabled_at", org.UID)
						errorCount++
						continue
					}
					updatedCount++
					logger.Infof("Set organisation %s disabled_at - subscription not active", org.UID)
				}
			}
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
