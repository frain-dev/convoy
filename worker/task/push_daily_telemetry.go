package task

import (
	"context"
	"time"

	"github.com/hibiken/asynq"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/configuration"
	"github.com/frain-dev/convoy/internal/events"
	"github.com/frain-dev/convoy/internal/organisations"
	"github.com/frain-dev/convoy/internal/projects"
	"github.com/frain-dev/convoy/internal/telemetry"
	log "github.com/frain-dev/convoy/pkg/logger"
)

const perPage = 50

func PushDailyTelemetry(lo log.Logger, db database.Database, locker JobLocker) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		// Pages every organisation, counts events per project, then posts to two
		// external telemetry backends; 15m covers the scan plus network waits.
		return locker.WithLock(ctx, "convoy:analytics:mutex", 15*time.Minute, func(ctx context.Context) error {
			orgRepo := organisations.New(lo, db)
			orgs, err := getAllOrganisations(ctx, orgRepo)
			if err != nil {
				return err
			}

			configRepo := configuration.New(lo, db)
			loadConfiguration, err := configRepo.LoadConfiguration(context.Background())
			if err != nil {
				return err
			}
			eventRepo := events.New(lo, db)
			projectRepo := projects.New(lo, db)

			totalEventsTracker := &telemetry.TotalEventsTracker{
				Orgs:        orgs,
				EventRepo:   eventRepo,
				ProjectRepo: projectRepo,
				Logger:      lo,
			}

			pb := telemetry.NewposthogBackend()
			mb := telemetry.NewmixpanelBackend()

			newTelemetry := telemetry.NewTelemetry(lo, loadConfiguration,
				telemetry.OptionTracker(totalEventsTracker),
				telemetry.OptionBackend(pb),
				telemetry.OptionBackend(mb))

			err = newTelemetry.Capture(ctx)
			if err != nil {
				return err
			}

			return nil
		})
	}
}

func getAllOrganisations(ctx context.Context, orgRepo datastore.OrganisationRepository) ([]datastore.Organisation, error) {
	var cursor = "FFFFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF"
	var orgs []datastore.Organisation

	for {
		paged, pagination, err := orgRepo.LoadOrganisationsPaged(ctx, datastore.Pageable{PerPage: perPage, NextCursor: cursor, Direction: datastore.Next})
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
