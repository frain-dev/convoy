package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/worker/task"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
)

type RetryEventDeliveryService struct {
	EventDeliveryRepo datastore.EventDeliveryRepository
	EndpointRepo      datastore.EndpointRepository
	Queue             queue.Queuer

	EventDelivery *datastore.EventDelivery
	Project       *datastore.Project
}

func (e *RetryEventDeliveryService) Run(ctx context.Context) error {
	switch e.EventDelivery.Status {
	case datastore.SuccessEventStatus:
		return errors.New("event already sent")
	case datastore.ScheduledEventStatus,
		datastore.ProcessingEventStatus,
		datastore.RetryEventStatus:
		return errors.New("cannot resend event that did not fail previously")
	}

	endpoint, err := e.EndpointRepo.FindEndpointByID(ctx, e.EventDelivery.EndpointID, e.Project.UID)
	if err != nil {
		return datastore.ErrEndpointNotFound
	}

	switch endpoint.Status {
	case datastore.PendingEndpointStatus:
		return errors.New("endpoint is being re-activated")
	case datastore.PausedEndpointStatus:
		return errors.New("endpoint is currently paused")
	case datastore.InactiveEndpointStatus:
		err = e.EndpointRepo.UpdateEndpointStatus(context.Background(), e.EventDelivery.ProjectID, e.EventDelivery.EndpointID, datastore.PendingEndpointStatus)
		if err != nil {
			log.FromContext(ctx).WithError(err).Error("failed to update endpoint status")
			return &ServiceError{ErrMsg: "failed to update endpoint status", Err: err}
		}
	}

	return requeueEventDelivery(ctx, e.EventDelivery, e.Project, e.EventDeliveryRepo, e.Queue)
}

func requeueEventDelivery(ctx context.Context, eventDelivery *datastore.EventDelivery, g *datastore.Project, ed datastore.EventDeliveryRepository, q queue.Queuer) error {
	eventDelivery.Status = datastore.ScheduledEventStatus
	err := ed.UpdateStatusOfEventDelivery(ctx, g.UID, *eventDelivery, datastore.ScheduledEventStatus)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to update event delivery status")
		return &ServiceError{ErrMsg: "an error occurred while trying to resend event", Err: err}
	}

	taskName := convoy.EventProcessor
	payload := task.EventDelivery{
		EventDeliveryID: eventDelivery.UID,
		ProjectID:       g.UID,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to marshal process event delivery payload")
		return &ServiceError{ErrMsg: "error occurred marshaling event delivery payload", Err: err}
	}

	job := &queue.Job{
		ID:      eventDelivery.UID,
		Payload: data,
		Delay:   1 * time.Second,
	}

	err = q.Write(taskName, convoy.EventQueue, job)
	if err != nil {
		log.FromContext(ctx).WithError(err).Errorf("error occurred re-enqueing old event - %s", eventDelivery.UID)
		return &ServiceError{ErrMsg: fmt.Sprintf("error occurred re-enqueing old event - %s", eventDelivery.UID), Err: err}
	}

	return nil
}
