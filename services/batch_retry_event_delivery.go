package services

import (
	"context"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
)

type BatchRetryEventDeliveryService struct {
	EventDeliveryRepo datastore.EventDeliveryRepository
	EndpointRepo      datastore.EndpointRepository
	Queue             queue.Queuer
	EventRepo         datastore.EventRepository

	Project *datastore.Project
	Filter  *datastore.EventDeliveryFilter
}

func (e *BatchRetryEventDeliveryService) Run(ctx context.Context) (int, int, error) {
	deliveries, _, err := e.EventDeliveryRepo.LoadEventDeliveriesPaged(ctx, e.Project.UID, e.Filter)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to fetch event deliveries by ids")
		return 0, 0, &ServiceError{ErrMsg: "failed to fetch event deliveries", Err: err}
	}

	r := RetryEventDeliveryService{
		EventDeliveryRepo: e.EventDeliveryRepo,
		EndpointRepo:      e.EndpointRepo,
		Queue:             e.Queue,
		Project:           e.Project,
	}

	failures := 0
	for _, delivery := range deliveries {
		r.EventDelivery = &delivery
		err := r.Run(ctx)
		if err != nil {
			failures++
			log.FromContext(ctx).WithError(err).Error("an item in the batch retry failed")
		}
	}

	successes := len(deliveries) - failures
	return successes, failures, nil
}
