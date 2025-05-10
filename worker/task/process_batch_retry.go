package task

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/hibiken/asynq"
	"gopkg.in/guregu/null.v4"
	"time"

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
		var batchRetry *datastore.BatchRetry
		err := msgpack.DecodeMsgPack(t.Payload(), &batchRetry)
		if err != nil {
			lo.WithError(err).Error("failed to unmarshal batch retry payload")
			return err
		}

		// Check if there's an active batch retry
		activeRetry, err := batchRetryRepo.FindActiveBatchRetry(ctx, batchRetry.ProjectID)
		if err != nil && err != datastore.ErrBatchRetryNotFound {
			lo.WithError(err).Error("failed to check for active batch retry")
			return err
		}

		if activeRetry != nil && activeRetry.ID != batchRetry.ID {
			return fmt.Errorf("an active batch retry already exists")
		}

		// Update status to processing
		batchRetry.Status = datastore.BatchRetryStatusProcessing
		err = batchRetryRepo.UpdateBatchRetry(ctx, batchRetry)
		if err != nil {
			lo.WithError(err).Error("failed to update batch retry status")
			return err
		}

		var totalProcessed, totalFailed int
		for {
			batchRetry.Filter.Pageable.PerPage = 1000

			// Load events in batches
			deliveries, pageable, innerErr := eventDeliveryRepo.LoadEventDeliveriesPaged(ctx,
				batchRetry.Filter.ProjectID,
				batchRetry.Filter.EndpointIDs,
				batchRetry.Filter.EventID,
				batchRetry.Filter.SubscriptionID,
				batchRetry.Filter.Status,
				batchRetry.Filter.SearchParams,
				batchRetry.Filter.Pageable,
				batchRetry.Filter.IdempotencyKey,
				batchRetry.Filter.EventType)
			if innerErr != nil {
				lo.WithError(innerErr).Error("failed to load deliveries")
				return innerErr
			}

			if innerErr != nil {
				now := time.Now()
				batchRetry.Status = datastore.BatchRetryStatusFailed
				batchRetry.Error = innerErr.Error()
				batchRetry.UpdatedAt = now
				batchRetry.CompletedAt = null.TimeFrom(now)
				_ = batchRetryRepo.UpdateBatchRetry(ctx, batchRetry)
				return innerErr
			}

			if len(deliveries) == 0 {
				break
			}

			// Process each event in the batch
			for _, delivery := range deliveries {
				// Queue the event delivery
				payload := EventDelivery{
					EventDeliveryID: delivery.UID,
					ProjectID:       batchRetry.ProjectID,
				}

				data, err2 := json.Marshal(payload)
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
			batchRetry.ProcessedEvents = totalProcessed
			batchRetry.FailedEvents = totalFailed
			batchRetry.UpdatedAt = time.Now()
			innerErr = batchRetryRepo.UpdateBatchRetry(ctx, batchRetry)
			if innerErr != nil {
				lo.WithError(innerErr).Error("failed to update batch retry progress")
			}

			if !pageable.HasNextPage {
				break
			}

			lo.Error("has next page ", "next_page_cursor: ", pageable.NextPageCursor)

			batchRetry.Filter.Pageable.NextCursor = pageable.NextPageCursor
			batchRetry.Filter.Pageable.Direction = datastore.Next
		}

		// Mark batch retry as completed
		now := time.Now()
		batchRetry.Status = datastore.BatchRetryStatusCompleted
		batchRetry.UpdatedAt = now
		batchRetry.CompletedAt = null.TimeFrom(now)
		err = batchRetryRepo.UpdateBatchRetry(ctx, batchRetry)
		if err != nil {
			lo.WithError(err).Error("failed to mark batch retry as completed")
			return err
		}

		return nil
	}
}
