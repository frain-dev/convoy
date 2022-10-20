package task

import (
	"context"
	"encoding/json"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/hibiken/asynq"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Payload struct {
	AppID      string `json:"app_id"`
	EndpointID string `json:"endpoint_id"`
	SecretID   string `json:"secret_id"`
}

func ExpireSecret(a datastore.ApplicationRepository) func(ctx context.Context, t *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		var payload Payload
		err := json.Unmarshal(t.Payload(), &payload)
		if err != nil {
			return &EndpointError{Err: err, delay: defaultDelay}
		}

		app, err := a.FindApplicationByID(ctx, payload.AppID)
		if err != nil {
			return &EndpointError{Err: err, delay: defaultDelay}
		}

		endpoint, err := app.FindEndpoint(payload.EndpointID)
		if err != nil {
			return &EndpointError{Err: err, delay: defaultDelay}
		}

		for _, secret := range endpoint.Secrets {
			if secret.UID == payload.SecretID && secret.DeletedAt == 0 {
				secret.DeletedAt = primitive.NewDateTimeFromTime(time.Now())
				break
			}
		}

		err = a.UpdateApplication(ctx, app, app.GroupID)
		if err != nil {
			return &EndpointError{Err: err, delay: defaultDelay}
		}

		return nil
	}
}
