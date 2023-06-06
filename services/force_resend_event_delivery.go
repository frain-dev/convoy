package services

import (
	"context"
	"errors"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
)

type ForceResendEventDeliveriesService struct {
	EventDeliveryRepo datastore.EventDeliveryRepository
	EndpointRepo      datastore.EndpointRepository
	Queue             queue.Queuer

	IDs     []string
	Project *datastore.Project
}

func (e *ForceResendEventDeliveriesService) Run(ctx context.Context) (int, int, error) {
	deliveries, err := e.EventDeliveryRepo.FindEventDeliveriesByIDs(ctx, e.Project.UID, e.IDs)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to fetch event deliveries by ids")
		return 0, 0, &ServiceError{ErrMsg: "failed to fetch event deliveries", Err: err}
	}

	err = validateEventDeliveryStatus(deliveries)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("event delivery status validation failed")
		return 0, 0, &ServiceError{ErrMsg: err.Error()}
	}

	failures := 0
	for _, delivery := range deliveries {
		err := e.forceResendEventDelivery(ctx, &delivery, e.Project)
		if err != nil {
			failures++
			log.FromContext(ctx).WithError(err).Error("an item in the force resend batch failed")
		}
	}

	successes := len(deliveries) - failures
	return successes, failures, nil
}

func (e *ForceResendEventDeliveriesService) forceResendEventDelivery(ctx context.Context, eventDelivery *datastore.EventDelivery, project *datastore.Project) error {
	endpoint, err := e.EndpointRepo.FindEndpointByID(ctx, eventDelivery.EndpointID, project.UID)
	if err != nil {
		return datastore.ErrEndpointNotFound
	}

	if endpoint.Status != datastore.ActiveEndpointStatus {
		return errors.New("force resend to an inactive or pending endpoint is not allowed")
	}

	return requeueEventDelivery(ctx, eventDelivery, project, e.EventDeliveryRepo, e.Queue)
}

func validateEventDeliveryStatus(deliveries []datastore.EventDelivery) error {
	for _, delivery := range deliveries {
		if delivery.Status != datastore.SuccessEventStatus {
			return ErrInvalidEventDeliveryStatus
		}
	}

	return nil
}
