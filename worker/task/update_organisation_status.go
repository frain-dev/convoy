package task

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/organisations"
	"github.com/frain-dev/convoy/internal/pkg/billing"
	log "github.com/frain-dev/convoy/pkg/logger"
)

const orgStatusUpdatePerPage = 50

func UpdateOrganisationStatus(db database.Database, billingClient billing.Client, locker JobLocker, logger log.Logger) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		if billingClient == nil {
			logger.Info("Billing client not configured, skipping organisation status update")
			return nil
		}

		cfg, err := config.Get()
		if err != nil {
			return fmt.Errorf("failed to get config: %w", err)
		}

		if !cfg.UsesOrgBilling() {
			logger.Info("cloud org billing is not configured, skipping organisation status update")
			return nil
		}

		return locker.WithLock(ctx, "convoy:update_organisation_status:mutex", time.Minute*30, func(ctx context.Context) error {
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
					logger.Errorf("Failed to fetch subscription for organisation %s: %v", org.UID, err)
					errorCount++
					continue
				}

				active := billing.HasActiveSubscription(resp.Data)
				if !billing.ApplySubscriptionStatus(&org, active) {
					continue
				}

				if err := orgRepo.UpdateOrganisation(ctx, &org); err != nil {
					if active {
						logger.Errorf("Failed to clear organisation %s disabled_at: %v", org.UID, err)
					} else {
						logger.Errorf("Failed to set organisation %s disabled_at: %v", org.UID, err)
					}
					errorCount++
					continue
				}
				updatedCount++
				if active {
					logger.Infof("Cleared organisation %s disabled_at - subscription active", org.UID)
				} else {
					logger.Infof("Set organisation %s disabled_at - subscription not active", org.UID)
				}
			}

			logger.Infof("Organisation status update completed: %d updated, %d errors", updatedCount, errorCount)
			return nil
		})
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
