package task

import (
	"context"
	"github.com/frain-dev/convoy/datastore"
	"github.com/hibiken/asynq"
)

func SearchTokenizer(eventRepo datastore.EventRepository) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		err := eventRepo.TokenizeEvents(ctx)
		if err != nil {
			return err
		}

		return nil
	}
}
