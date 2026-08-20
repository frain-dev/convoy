package services

import (
	"context"
	"fmt"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/notifications"
	"github.com/frain-dev/convoy/internal/pkg/license"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/queue"
)

type ActivateEndpointService struct {
	EndpointRepo datastore.EndpointRepository
	Queue        queue.Queuer
	Licenser     license.Licenser
	Project      *datastore.Project
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

	wasDisabled := endpoint.Status == datastore.InactiveEndpointStatus

	changed, err := s.EndpointRepo.UpdateEndpointStatus(ctx, s.ProjectID, endpoint.UID, datastore.ActiveEndpointStatus)
	if err != nil {
		s.Logger.ErrorContext(ctx, "failed to activate endpoint", "error", err)
		return nil, &ServiceError{ErrMsg: "failed to activate endpoint", Err: err}
	}

	// The all-clear that answers the disable alert. Only endpoints that were
	// disabled get it: a paused endpoint was never reported down. Gated on the
	// write having changed the status so two concurrent activations announce it
	// once, and on AdvancedEndpointMgmt like disable alerts.
	if wasDisabled && changed && s.Licenser != nil && s.Licenser.AdvancedEndpointMgmt() {
		notifications.SendEndpointNotification(ctx, endpoint, s.Project, datastore.ActiveEndpointStatus, s.Queue, false, "", "", 0, s.Logger)
	}

	// Reflect the persisted status; returning the pre-update snapshot makes
	// clients that trust the response render a stale "inactive".
	endpoint.Status = datastore.ActiveEndpointStatus

	return endpoint, nil
}
