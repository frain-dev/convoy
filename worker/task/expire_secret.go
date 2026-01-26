package task

import (
	"context"
	"encoding/json"

	"github.com/olamilekan000/surge/surge/job"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/msgpack"
)

type Payload struct {
	EndpointID string `json:"endpoint_id"`
	SecretID   string `json:"secret_id"`
	ProjectID  string `json:"project_id"`
}

func ExpireSecret(a datastore.EndpointRepository) func(ctx context.Context, jobEnvelope *job.JobEnvelope) error {
	return func(ctx context.Context, jobEnvelope *job.JobEnvelope) error {
		var payload Payload

		err := msgpack.DecodeMsgPack(jobEnvelope.Args, &payload)
		if err != nil {
			err := json.Unmarshal(jobEnvelope.Args, &payload)
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
