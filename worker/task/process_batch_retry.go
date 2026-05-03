package task

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/frain-dev/convoy/queue"
)

// getOrDefault returns the value if it's not the zero value, otherwise returns the default
func getOrDefault[T comparable](value, defaultValue T) T {
	var zero T
	if value == zero {
		return defaultValue
	}
	return value
}

func ProcessBatchRetry(
	batchRetryRepo datastore.BatchRetryRepository,
	eventDeliveryRepo datastore.EventDeliveryRepository,
	queuer queue.Queuer,
	lo log.Logger,
) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		var br *datastore.BatchRetry
		err := msgpack.DecodeMsgPack(t.Payload(), &br)
		if err != nil {
			lo.Error("failed to unmarshal batch retry payload", "error", err)
			return err
		}

		// Check if there's an active batch retry
		activeRetry, err := batchRetryRepo.FindActiveBatchRetry(ctx, br.ProjectID)
		if err != nil && !errors.Is(err, datastore.ErrBatchRetryNotFound) {
			lo.Error("failed to check for active batch retry", "error", err)
			return err
		}

		// If no active batch retry found, use the one from the task payload
		if activeRetry == nil {
			activeRetry = br
		} else if activeRetry.ID != br.ID {
			return fmt.Errorf("an active batch retry already exists")
		}

		// Ensure the batch retry has a valid filter
		if activeRetry.Filter == nil {
			return fmt.Errorf("batch retry has no filter")
		}

		// Update status to processing
		activeRetry.Status = datastore.BatchRetryStatusProcessing
		err = batchRetryRepo.UpdateBatchRetry(ctx, activeRetry)
		if err != nil {
			lo.Error("failed to update batch retry status", "error", err)
			return err
		}

		totalProcessed := activeRetry.ProcessedEvents
		totalFailed := activeRetry.FailedEvents

		for {
			activeRetry, err = batchRetryRepo.FindActiveBatchRetry(ctx, br.ProjectID)
			if err != nil && !errors.Is(err, datastore.ErrBatchRetryNotFound) {
				lo.Error("failed to check for active batch retry", "error", err)
				return err
			}

			f, filterErr := activeRetry.GetFilter()
			if filterErr != nil {
				lo.Error("failed to get filter", "error", filterErr)
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
					PerPage:    getOrDefault(f.Pageable.PerPage, 1000),
					Direction:  getOrDefault(f.Pageable.Direction, datastore.Next),
					Sort:       f.Pageable.Sort,
					PrevCursor: f.Pageable.PrevCursor,
					NextCursor: getOrDefault(f.Pageable.NextCursor, datastore.DefaultCursor),
				},
				IdempotencyKey: f.IdempotencyKey,
				Status:         f.Status,
				SearchParams:   f.SearchParams,
			}

			lo.Info("start of loop", "next_page_cursor", filter)

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
				filter.EventType,
				filter.BrokerMessageId)
			if innerErr != nil {
				lo.Error("failed to load deliveries", "error", innerErr)
				now := time.Now()
				activeRetry.Status = datastore.BatchRetryStatusFailed
				activeRetry.Error = innerErr.Error()
				activeRetry.UpdatedAt = now
				activeRetry.CompletedAt = null.TimeFrom(now)
				filterErr = batchRetryRepo.UpdateBatchRetry(ctx, activeRetry)
				if filterErr != nil {
					lo.Error("failed to mark batch retry as failed", "error", filterErr)
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
					ID:      delivery.UID,
					Payload: data,
				}

				err2 = queuer.Write(ctx, convoy.EventProcessor, convoy.EventQueue, job)
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
				lo.Error("failed to update batch retry progress", "error", innerErr)
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
			lo.Error("failed to mark batch retry as completed", "error", err)
			return err
		}

		return nil
	}
}
