package services

import (
	"context"
	"fmt"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
)

type ToggleEndpointStatusService struct {
	EndpointRepo datastore.EndpointRepository
	ProjectID    string
	EndpointId   string
}

func (s *ToggleEndpointStatusService) Run(ctx context.Context) (*datastore.Endpoint, error) {
	endpoint, err := s.EndpointRepo.FindEndpointByID(ctx, s.EndpointId, s.ProjectID)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error(ErrSubscriptionNotFound.Error())
		return nil, &ServiceError{ErrMsg: "failed to find endpoint", Err: err}
	}

	switch endpoint.Status {
	case datastore.ActiveEndpointStatus:
		endpoint.Status = datastore.InactiveEndpointStatus
	case datastore.InactiveEndpointStatus:
		endpoint.Status = datastore.ActiveEndpointStatus
	case datastore.PendingEndpointStatus:
		return nil, &ServiceError{ErrMsg: "endpoint is in pending status"}
	default:
		return nil, &ServiceError{ErrMsg: fmt.Sprintf("unknown endpoint status: %s", endpoint.Status)}
	}

	err = s.EndpointRepo.UpdateEndpointStatus(ctx, s.ProjectID, endpoint.UID, endpoint.Status)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to update endpoint status")
		return nil, &ServiceError{ErrMsg: "failed to update endpoint status", Err: err}
	}

	return endpoint, nil
}
