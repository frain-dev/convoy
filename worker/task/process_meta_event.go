package task

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/hibiken/asynq"
)

type MetaEvent struct {
	Event *datastore.MetaEvent
}

func ProcessMetaEvent(projectRepo datastore.ProjectRepository, metaEventRepo datastore.MetaEventRepository) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		var data MetaEvent

		err := json.Unmarshal(t.Payload(), &data)
		if err != nil {
			log.WithError(err).Error("failed to unmarshal process process meta event payload")
			return &EndpointError{Err: err, delay: defaultDelay}
		}

		// switch meta event status to processing

		fmt.Println("INSIDE WORKER >>>>>")
		return nil
	}
}
