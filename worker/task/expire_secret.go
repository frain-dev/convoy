package task

import (
	"context"
	"encoding/json"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	"github.com/hibiken/asynq"
)

type Payload struct {
	EndpointID string `json:"endpoint_id"`
	SecretID   string `json:"secret_id"`
}

func ExpireSecret(a datastore.EndpointRepository) func(ctx context.Context, t *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		var payload Payload
		err := json.Unmarshal(t.Payload(), &payload)
		if err != nil {
			return &EndpointError{Err: err, delay: defaultDelay}
		}

		endpoint, err := a.FindEndpointByID(ctx, payload.EndpointID)
		if err != nil {
			return &EndpointError{Err: err, delay: defaultDelay}
		}

		for _, secret := range endpoint.Secrets {
			if secret.UID == payload.SecretID && secret.DeletedAt == nil {
				secret.DeletedAt = util.NewDateTime()
				break
			}
		}

		err = a.UpdateEndpoint(ctx, endpoint, endpoint.ProjectID)
		if err != nil {
			return &EndpointError{Err: err, delay: defaultDelay}
		}

		return nil
	}
}
