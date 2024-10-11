package services

import (
	"context"
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

	if endpoint.Status != datastore.InactiveEndpointStatus {
		return nil, &ServiceError{ErrMsg: "the endpoint must be inactive"}
	}

	err = s.EndpointRepo.UpdateEndpointStatus(ctx, s.ProjectID, endpoint.UID, datastore.ActiveEndpointStatus)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to activate endpoint")
		return nil, &ServiceError{ErrMsg: "failed to activate endpoint", Err: err}
	}

	return endpoint, nil
}
