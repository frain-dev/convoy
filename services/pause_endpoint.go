package services

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/frain-dev/convoy/datastore"
)

type PauseEndpointService struct {
	EndpointRepo datastore.EndpointRepository
	ProjectID    string
	EndpointId   string
}

func (s *PauseEndpointService) Run(ctx context.Context) (*datastore.Endpoint, error) {
	endpoint, err := s.EndpointRepo.FindEndpointByID(ctx, s.EndpointId, s.ProjectID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to find endpoint", "error", err)
		return nil, &ServiceError{ErrMsg: "failed to find endpoint", Err: err}
	}

	switch endpoint.Status {
	case datastore.ActiveEndpointStatus:
		endpoint.Status = datastore.PausedEndpointStatus
	case datastore.PausedEndpointStatus:
		endpoint.Status = datastore.ActiveEndpointStatus
	default:
		return nil, &ServiceError{ErrMsg: fmt.Sprintf("current endpoint status - %s, does not support pausing or unpausing", endpoint.Status)}
	}

	err = s.EndpointRepo.UpdateEndpointStatus(ctx, s.ProjectID, endpoint.UID, endpoint.Status)
	if err != nil {
		slog.ErrorContext(ctx, "failed to update endpoint status", "error", err)
		return nil, &ServiceError{ErrMsg: "failed to update endpoint status", Err: err}
	}

	return endpoint, nil
}
