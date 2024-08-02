package task

import (
	"context"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/stats"
	"github.com/hibiken/asynq"
)

func RecordStats(deliveryRepo datastore.EventDeliveryRepository, configRepo datastore.ConfigurationRepository) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		s := stats.NewStats(deliveryRepo, configRepo)
		err := s.Record(ctx)
		if err != nil {
			return err
		}

		return nil
	}
}
