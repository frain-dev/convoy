package services

import (
	"context"
	"fmt"

	"github.com/frain-dev/convoy/datastore"
	log "github.com/frain-dev/convoy/pkg/logger"
)

type ActivateEndpointService struct {
	EndpointRepo datastore.EndpointRepository
	ProjectID    string
	EndpointId   string
	Logger       log.Logger
}

func (s *ActivateEndpointService) Run(ctx context.Context) (*datastore.Endpoint, error) {
	endpoint, err := s.EndpointRepo.FindEndpointByID(ctx, s.EndpointId, s.ProjectID)
	if err != nil {
		s.Logger.ErrorContext(ctx, "failed to find endpoint", "error", err)
		return nil, &ServiceError{ErrMsg: "failed to find endpoint", Err: err}
	}

	if endpoint.Status != datastore.InactiveEndpointStatus && endpoint.Status != datastore.PausedEndpointStatus {
		return nil, &ServiceError{ErrMsg: fmt.Sprintf("current endpoint status - %s, does not support activation", endpoint.Status)}
	}

	err = s.EndpointRepo.UpdateEndpointStatus(ctx, s.ProjectID, endpoint.UID, datastore.ActiveEndpointStatus)
	if err != nil {
		s.Logger.ErrorContext(ctx, "failed to activate endpoint", "error", err)
		return nil, &ServiceError{ErrMsg: "failed to activate endpoint", Err: err}
	}

	return endpoint, nil
}
