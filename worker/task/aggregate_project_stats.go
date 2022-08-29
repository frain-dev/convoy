package task

import (
	"context"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/hibiken/asynq"
	log "github.com/sirupsen/logrus"
)

func AggregateProjectStats(groupRepo datastore.GroupRepository) func(ctx context.Context, t *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		groups, err := groupRepo.LoadGroups(ctx, &datastore.GroupFilter{OrgID: ""})
		if err != nil {
			log.WithError(err).Error("failed to load groups")
			return &EndpointError{Err: err, delay: time.Minute}
		}

		err = groupRepo.FillGroupsStatistics(ctx, groups)
		if err != nil {
			log.WithError(err).Error("failed to persist project statistics")
			return &EndpointError{Err: err, delay: time.Minute}
		}

		log.Println("saved project stats")

		return nil
	}
}
