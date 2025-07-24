package deleter

import (
	"context"
	"fmt"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"time"
)

type DeleteRetentionPolicy struct {
	logger log.StdLogger
	db     database.Database
	policy time.Duration
}

func (d *DeleteRetentionPolicy) Maintain(ctx context.Context) error {
	eventRepo := postgres.NewEventRepo(d.db)
	projectRepo := postgres.NewProjectRepo(d.db)
	eventDeliveryRepo := postgres.NewEventDeliveryRepo(d.db)
	deliveryAttemptsRepo := postgres.NewDeliveryAttemptRepo(d.db)

	filter := &datastore.ProjectFilter{}
	projects, err := projectRepo.LoadProjects(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to load projects: %w", err)
	}

	if len(projects) == 0 {
		d.logger.Warn("no existing projects, retention policy job exiting")
		return nil
	}

	for _, p := range projects {
		deliveryFilter := &datastore.DeliveryAttemptsFilter{
			CreatedAtStart: 0,
			CreatedAtEnd:   time.Now().Add(-d.policy).Unix(),
		}

		err = deliveryAttemptsRepo.DeleteProjectDeliveriesAttempts(ctx, p.UID, deliveryFilter, true)
		if err != nil {
			d.logger.WithError(err).Errorf("failed to delete project delivery attempts for project: %s", p.UID)
		}

		eventDeliveryFilter := &datastore.EventDeliveryFilter{
			CreatedAtStart: 0,
			CreatedAtEnd:   time.Now().Add(-d.policy).Unix(),
		}

		err = eventDeliveryRepo.DeleteProjectEventDeliveries(ctx, p.UID, eventDeliveryFilter, true)
		if err != nil {
			d.logger.WithError(err).Errorf("failed to delete project event deliveries for project: %s", p.UID)
		}

		eventFilter := &datastore.EventFilter{
			CreatedAtStart: 0,
			CreatedAtEnd:   time.Now().Add(-d.policy).Unix(),
		}

		err = eventRepo.DeleteProjectEvents(ctx, p.UID, eventFilter, true)
		if err != nil {
			d.logger.WithError(err).Errorf("failed to delete project events for project: %s", p.UID)
		}

		err = eventRepo.DeleteProjectTokenizedEvents(ctx, p.UID, eventFilter)
		if err != nil {
			d.logger.WithError(err).Errorf("failed to delete tokenized project events for project: %s", p.UID)
		}
	}

	return nil
}

func (d *DeleteRetentionPolicy) Start(_ context.Context, _ time.Duration) {}

func NewDeleteRetentionPolicy(db database.Database, logger log.StdLogger, policy time.Duration) *DeleteRetentionPolicy {
	return &DeleteRetentionPolicy{
		logger: logger,
		policy: policy,
		db:     db,
	}
}
