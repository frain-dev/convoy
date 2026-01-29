package services

import (
	"context"
	"fmt"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
)

type ActivateEndpointService struct {
	EndpointRepo datastore.EndpointRepository
	ProjectID    string
	EndpointId   string
}

func (s *ActivateEndpointService) Run(ctx context.Context) (*datastore.Endpoint, error) {
	endpoint, err := s.EndpointRepo.FindEndpointByID(ctx, s.EndpointId, s.ProjectID)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to find endpoint")
		return nil, &ServiceError{ErrMsg: "failed to find endpoint", Err: err}
	}

	if endpoint.Status != datastore.InactiveEndpointStatus && endpoint.Status != datastore.PausedEndpointStatus {
		return nil, &ServiceError{ErrMsg: fmt.Sprintf("current endpoint status - %s, does not support activation", endpoint.Status)}
	}

	err = s.EndpointRepo.UpdateEndpointStatus(ctx, s.ProjectID, endpoint.UID, datastore.ActiveEndpointStatus)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to activate endpoint")
		return nil, &ServiceError{ErrMsg: "failed to activate endpoint", Err: err}
	}

	return endpoint, nil
}
