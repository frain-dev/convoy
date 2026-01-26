package services

import (
	"context"
	"errors"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/frain-dev/convoy/queue"
)

type BatchRetryEventDeliveryService struct {
	BatchRetryRepo    datastore.BatchRetryRepository
	EventDeliveryRepo datastore.EventDeliveryRepository
	Queue             queue.Queuer
	Filter            *datastore.Filter
	ProjectID         string
}

func (e *BatchRetryEventDeliveryService) Run(ctx context.Context) error {
	// Check if there's an active batch retry
	activeRetry, err := e.BatchRetryRepo.FindActiveBatchRetry(ctx, e.ProjectID)
	if err != nil && !errors.Is(err, datastore.ErrBatchRetryNotFound) {
		log.FromContext(ctx).WithError(err).Error("failed to check for active batch retry")
		return &ServiceError{ErrMsg: "failed to check for active batch retry", Err: err}
	}

	if activeRetry != nil {
		return &ServiceError{ErrMsg: "an active batch retry already exists"}
	}

	// Count total events
	count, err := e.EventDeliveryRepo.CountEventDeliveries(ctx, e.ProjectID, e.Filter.EndpointIDs, e.Filter.EventID, e.Filter.Status, e.Filter.SearchParams)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to count events")
		return &ServiceError{ErrMsg: "failed to count events", Err: err}
	}

	// Create a batch retry record
	batchRetry := &datastore.BatchRetry{
		ID:              ulid.Make().String(),
		ProjectID:       e.ProjectID,
		Status:          datastore.BatchRetryStatusPending,
		TotalEvents:     int(count),
		Filter:          datastore.FromFilterStruct(*e.Filter),
		ProcessedEvents: 0,
		FailedEvents:    0,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	err = e.BatchRetryRepo.CreateBatchRetry(ctx, batchRetry)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to create batch retry")
		return &ServiceError{ErrMsg: "failed to create batch retry", Err: err}
	}

	data, err := msgpack.EncodeMsgPack(batchRetry)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to encode batch retry payload")
		return &ServiceError{ErrMsg: "failed to encode batch retry payload", Err: err}
	}

	jobID := queue.JobId{
		ProjectID:  e.ProjectID,
		ResourceID: batchRetry.ID,
	}.BatchRetryJobId()

	job := &queue.Job{
		ID:      jobID,
		Payload: data,
		Delay:   0,
	}

	err = e.Queue.WriteWithoutTimeout(convoy.BatchRetryProcessor, convoy.BatchRetryQueue, job)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to queue batch retry job")
		return &ServiceError{ErrMsg: "failed to queue batch retry job", Err: err}
	}

	return nil
}
