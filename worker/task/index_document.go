package task

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/frain-dev/convoy/internal/pkg/searcher"
	"github.com/hibiken/asynq"
)

var (
	ErrProjectIdFieldIsRequired  = errors.New("project_id field should be a string")
	ErrProjectIdFieldIsNotString = errors.New("project_id field does not exist on the document")
)

func SearchIndex(search searcher.Searcher) func(ctx context.Context, t *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		if search == nil {
			return nil
		}

		buf := t.Payload()

		var event map[string]interface{}
		err := json.Unmarshal(buf, &event)
		if err != nil {
			return &EndpointError{Err: err, delay: defaultDelay}
		}

		event["id"] = event["uid"]
		if g, found := event["project_id"]; found {
			if project_id, ok := g.(string); ok {
				err = search.Index(project_id, event)
				if err != nil {
					return &EndpointError{Err: err, delay: time.Second * 5}
				}
			} else {
				return &EndpointError{Err: ErrProjectIdFieldIsNotString, delay: time.Second * 1}
			}
		} else {
			return &EndpointError{Err: ErrProjectIdFieldIsRequired, delay: time.Second * 1}
		}

		return nil
	}
}
