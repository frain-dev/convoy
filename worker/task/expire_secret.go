package task

import (
	"context"
	"encoding/json"
	"github.com/frain-dev/convoy/util"

	"github.com/frain-dev/convoy/datastore"
	"github.com/hibiken/asynq"
)

type Payload struct {
	EndpointID string `json:"endpoint_id"`
	SecretID   string `json:"secret_id"`
	ProjectID  string `json:"project_id"`
}

func ExpireSecret(a datastore.EndpointRepository) func(ctx context.Context, t *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		var payload Payload

		err := util.DecodeMsgPack(t.Payload(), &payload)
		if err != nil {
			err := json.Unmarshal(t.Payload(), &payload)
			if err != nil {
				return &EndpointError{Err: err, delay: defaultDelay}
			}
		}

		endpoint, err := a.FindEndpointByID(ctx, payload.EndpointID, payload.ProjectID)
		if err != nil {
			return &EndpointError{Err: err, delay: defaultDelay}
		}

		err = a.DeleteSecret(ctx, endpoint, payload.SecretID, payload.ProjectID)
		if err != nil {
			return &EndpointError{Err: err, delay: defaultDelay}
		}

		return nil
	}
}
