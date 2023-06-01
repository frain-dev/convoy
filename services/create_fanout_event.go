package services

import (
	"context"
	"errors"
	"net/http"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/util"
)

type CreateFanoutEventService struct {
	EndpointRepo datastore.EndpointRepository
	Queue        queue.Queuer

	NewMessage *models.FanoutEvent
	Project    *datastore.Project
}

func (e *CreateFanoutEventService) Run(ctx context.Context) (*datastore.Event, error) {
	if e.Project == nil {
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("an error occurred while creating event - invalid project"))
	}

	if err := util.Validate(e.NewMessage); err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	endpoints, err := e.EndpointRepo.FindEndpointsByOwnerID(ctx, e.Project.UID, e.NewMessage.OwnerID)
	if err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	if len(endpoints) == 0 {
		return nil, util.NewServiceError(http.StatusBadRequest, ErrNoValidOwnerIDEndpointFound)
	}

	ev := &newEvent{
		Data:          e.NewMessage.Data,
		EventType:     e.NewMessage.EventType,
		Raw:           string(e.NewMessage.Data),
		CustomHeaders: e.NewMessage.CustomHeaders,
	}

	event, err := createEvent(ctx, endpoints, ev, e.Project, e.Queue)
	if err != nil {
		return nil, err
	}

	return event, nil
}
