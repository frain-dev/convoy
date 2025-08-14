package task

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/hibiken/asynq"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
)

func ProcessBatchRetry(
	batchRetryRepo datastore.BatchRetryRepository,
	eventDeliveryRepo datastore.EventDeliveryRepository,
	queuer queue.Queuer,
	lo *log.Logger,
) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		var br *datastore.BatchRetry
		err := msgpack.DecodeMsgPack(t.Payload(), &br)
		if err != nil {
			lo.WithError(err).Error("failed to unmarshal batch retry payload")
			return err
		}

		// Check if there's an active batch retry
		activeRetry, err := batchRetryRepo.FindActiveBatchRetry(ctx, br.ProjectID)
		if err != nil && !errors.Is(err, datastore.ErrBatchRetryNotFound) {
			lo.WithError(err).Error("failed to check for active batch retry")
			return err
		}

		if activeRetry != nil && activeRetry.ID != br.ID {
			return fmt.Errorf("an active batch retry already exists")
		}

		// Update status to processing
		activeRetry.Status = datastore.BatchRetryStatusProcessing
		err = batchRetryRepo.UpdateBatchRetry(ctx, activeRetry)
		if err != nil {
			lo.WithError(err).Error("failed to update batch retry status")
			return err
		}

		totalProcessed := activeRetry.ProcessedEvents
		totalFailed := activeRetry.FailedEvents

		for {
			activeRetry, err = batchRetryRepo.FindActiveBatchRetry(ctx, br.ProjectID)
			if err != nil && !errors.Is(err, datastore.ErrBatchRetryNotFound) {
				lo.WithError(err).Error("failed to check for active batch retry")
				return err
			}

			f, filterErr := activeRetry.GetFilter()
			if filterErr != nil {
				lo.WithError(filterErr).Error("failed to get filter")
				return filterErr
			}

			filter := &datastore.Filter{
				Query:          f.Query,
				OwnerID:        f.OwnerID,
				Project:        f.Project,
				ProjectID:      f.ProjectID,
				EndpointID:     f.EndpointID,
				EndpointIDs:    f.EndpointIDs,
				SubscriptionID: f.SubscriptionID,
				EventID:        f.EventID,
				EventType:      f.EventType,
				SourceID:       f.SourceID,
				SourceIDs:      f.SourceIDs,
				Pageable: datastore.Pageable{
					PerPage:    f.Pageable.PerPage,
					Direction:  f.Pageable.Direction,
					Sort:       f.Pageable.Sort,
					PrevCursor: f.Pageable.PrevCursor,
					NextCursor: f.Pageable.NextCursor,
				},
				IdempotencyKey: f.IdempotencyKey,
				Status:         f.Status,
				SearchParams:   f.SearchParams,
			}

			lo.WithFields(map[string]interface{}{"next_page_cursor": filter}).Info("start of loop")

			// Load events in batches
			deliveries, pgData, innerErr := eventDeliveryRepo.LoadEventDeliveriesPaged(ctx,
				filter.ProjectID,
				filter.EndpointIDs,
				filter.EventID,
				filter.SubscriptionID,
				filter.Status,
				filter.SearchParams,
				filter.Pageable,
				filter.IdempotencyKey,
				filter.EventType)
			if innerErr != nil {
				lo.WithError(innerErr).Error("failed to load deliveries")
				now := time.Now()
				activeRetry.Status = datastore.BatchRetryStatusFailed
				activeRetry.Error = innerErr.Error()
				activeRetry.UpdatedAt = now
				activeRetry.CompletedAt = null.TimeFrom(now)
				filterErr = batchRetryRepo.UpdateBatchRetry(ctx, activeRetry)
				if filterErr != nil {
					lo.WithError(filterErr).Error("failed to mark batch retry as failed")
				}
				return errors.Join(filterErr, innerErr)
			}

			if len(deliveries) == 0 {
				lo.Info("no deliveries received from db, exiting")
				break
			}

			// Process each event in the batch
			for _, delivery := range deliveries {
				// Queue the event delivery
				payload := EventDelivery{
					EventDeliveryID: delivery.UID,
					ProjectID:       activeRetry.ProjectID,
				}

				data, err2 := msgpack.EncodeMsgPack(payload)
				if err2 != nil {
					totalFailed++
					continue
				}

				job := &queue.Job{
					Payload: data,
					Delay:   0,
				}

				err2 = queuer.Write(convoy.EventProcessor, convoy.EventQueue, job)
				if err2 != nil {
					totalFailed++
					continue
				}

				totalProcessed++
			}

			// Update progress
			activeRetry.ProcessedEvents = totalProcessed
			activeRetry.FailedEvents = totalFailed
			activeRetry.UpdatedAt = time.Now()

			filter.Pageable = datastore.Pageable{
				PerPage:    filter.Pageable.PerPage,
				Direction:  filter.Pageable.Direction,
				Sort:       filter.Pageable.Sort,
				PrevCursor: filter.Pageable.PrevCursor,
				NextCursor: pgData.NextPageCursor,
			}

			activeRetry.Filter = datastore.FromFilterStruct(*filter)

			innerErr = batchRetryRepo.UpdateBatchRetry(ctx, activeRetry)
			if innerErr != nil {
				lo.WithError(innerErr).Error("failed to update batch retry progress")
			}

			if !pgData.HasNextPage {
				break
			}
		}

		// Mark batch retry as completed
		now := time.Now()
		activeRetry.Status = datastore.BatchRetryStatusCompleted
		activeRetry.UpdatedAt = now
		activeRetry.CompletedAt = null.TimeFrom(now)
		err = batchRetryRepo.UpdateBatchRetry(ctx, activeRetry)
		if err != nil {
			lo.WithError(err).Error("failed to mark batch retry as completed")
			return err
		}

		return nil
	}
}
