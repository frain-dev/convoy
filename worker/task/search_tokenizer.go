package task

import (
	"context"
	"fmt"
	"github.com/frain-dev/convoy/datastore"
	"github.com/hibiken/asynq"
)

func SearchTokenizer(projectRepository datastore.ProjectRepository) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		projectEvents, err := projectRepository.GetProjectsWithEvents(ctx, "")
		if err != nil {
			return err
		}

		for _, p := range projectEvents {
			fmt.Printf("%+v\n", p)
		}

		return nil
	}
}
