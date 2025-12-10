package task

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/batch_tracker"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/frain-dev/convoy/queue"
	redisqueue "github.com/frain-dev/convoy/queue/redis"
	"github.com/frain-dev/convoy/util"
)

func RetryEventDeliveries(db database.Database, eventQueue queue.Queuer, statuses []datastore.EventDeliveryStatus, lookBackDuration, eventId string) {
	RetryEventDeliveriesWithTracker(db, eventQueue, statuses, lookBackDuration, eventId, "", nil)
}

func RetryEventDeliveriesWithTracker(db database.Database, eventQueue queue.Queuer, statuses []datastore.EventDeliveryStatus, lookBackDuration, eventId, batchID string, tracker *batch_tracker.BatchTracker) {
	if len(statuses) == 1 && util.IsStringEmpty(string(statuses[0])) {
		statuses = []datastore.EventDeliveryStatus{"Retry", "Scheduled", "Processing"}
	}

	if util.IsStringEmpty(lookBackDuration) {
		// TODO(subomi): Setup configuration
		lookBackDuration = "5h"
	}

	d, err := time.ParseDuration(lookBackDuration)
	if err != nil {
		log.Error("Failed to parse time duration")
	}
	now := time.Now()
	then := now.Add(-d)

	ctx := context.Background()

	// Initialize repositories and queue once
	eventDeliveryRepo := postgres.NewEventDeliveryRepo(db)
	var q *redisqueue.RedisQueue
	q, ok := eventQueue.(*redisqueue.RedisQueue)
	if !ok {
		log.Errorf("Invalid queue type for requeing event deliveries: %T", eventQueue)
		return
	}

	var allStatusesWg sync.WaitGroup

	// Initialize batch tracking (we'll update total count as we process)
	if tracker != nil && batchID != "" {
		// Build status filter string - show all statuses if multiple, or single if one
		statusFilter := ""
		if len(statuses) == 1 {
			statusFilter = string(statuses[0])
		} else if len(statuses) > 1 {
			// Join all statuses with comma
			statusStrings := make([]string, len(statuses))
			for i, s := range statuses {
				statusStrings[i] = string(s)
			}
			statusFilter = strings.Join(statusStrings, ", ")
		}

		// Start with 0, we'll update the total as we actually process deliveries
		if err := tracker.CreateBatch(ctx, batchID, 0, statusFilter, lookBackDuration, eventId); err != nil {
			log.WithError(err).Error("Failed to create batch tracker")
		}
	}

	for _, status := range statuses {
		allStatusesWg.Add(1)
		go func(s datastore.EventDeliveryStatus) {
			defer allStatusesWg.Done()
			log.Printf("Searching for events with status %s", s)
			searchParams := datastore.SearchParams{
				CreatedAtStart: then.Unix(),
				CreatedAtEnd:   now.Unix(),
			}

			pageable := datastore.Pageable{
				Direction:  datastore.Next,
				PerPage:    1000,
				NextCursor: datastore.DefaultCursor,
			}

			deliveryChan := make(chan []datastore.EventDelivery, 4)
			count := 0

			var wg sync.WaitGroup

			wg.Add(1)

			go processEventDeliveryBatch(ctx, s, eventDeliveryRepo, deliveryChan, q, &wg, batchID, tracker)

			counter, err := eventDeliveryRepo.CountDeliveriesByStatus(ctx, "", s, searchParams)
			if err != nil {
				log.Error("Failed to count event deliveries")
			}
			log.Infof("Total number of event deliveries to requeue is %d", counter)

			for {
				deliveries, pagination, err := eventDeliveryRepo.LoadEventDeliveriesPaged(ctx, "", []string{}, eventId, "", []datastore.EventDeliveryStatus{s}, searchParams, pageable, "", "", "")
				if err != nil {
					log.WithError(err).Errorf("successfully fetched %d event deliveries but with error", count)
					close(deliveryChan)
					log.Info("closed delivery channel")
					break
				}

				// stop when len(deliveries) is 0
				if len(deliveries) == 0 {
					log.Warn("no deliveries received from db, exiting")
					close(deliveryChan)
					log.Info("closed delivery channel")
					break
				}

				count += len(deliveries)
				deliveryChan <- deliveries
				pageable.NextCursor = pagination.NextPageCursor
			}

			log.Info("waiting for batch processor to finish")
			wg.Wait()
		}(status)
	}

	// Wait for all status processing to complete
	allStatusesWg.Wait()

	// Complete batch tracking if tracker is provided (after all statuses are processed)
	if tracker != nil && batchID != "" {
		if err := tracker.SyncCounters(ctx, batchID); err != nil {
			log.WithError(err).Error("Failed to sync batch counters")
		}
		if err := tracker.CompleteBatch(ctx, batchID); err != nil {
			log.WithError(err).Error("Failed to complete batch tracking")
		}
	}
}

func processEventDeliveryBatch(ctx context.Context, status datastore.EventDeliveryStatus, eventDeliveryRepo datastore.EventDeliveryRepository, deliveryChan <-chan []datastore.EventDelivery, q *redisqueue.RedisQueue, wg *sync.WaitGroup, batchID string, tracker *batch_tracker.BatchTracker) {
	defer wg.Done()

	batchCount := 1
	for {
		// ok will return false if the channel is closed and drained(empty), at which point
		// we should return
		batch, ok := <-deliveryChan
		if !ok {
			// the channel has been closed and there are no more deliveries coming in
			log.Warn("batch processor exiting")
			return
		}

		batchIDs := make([]string, len(batch))
		for i := range batch {
			batchIDs[i] = batch[i].UID
		}

		if status == datastore.ProcessingEventStatus {
			err := eventDeliveryRepo.UpdateStatusOfEventDeliveries(ctx, "", batchIDs, datastore.ScheduledEventStatus)
			if err != nil {
				log.WithError(err).Errorf("batch %d: failed to update event deliveries status", batchCount)
			}
		}

		// remove these event deliveries queue
		err := q.DeleteEventDeliveriesFromQueue(convoy.EventQueue, batchIDs)
		if err != nil {
			log.WithError(err).WithField("ids", batchIDs).Errorf("batch %d: failed to delete event deliveries from zset", batchCount)
		}

		processedInBatch := int64(0)
		failedInBatch := int64(0)

		for i := range batch {
			delivery := &batch[i]

			payload := EventDelivery{
				EventDeliveryID: delivery.UID,
				ProjectID:       delivery.ProjectID,
			}

			data, err := msgpack.EncodeMsgPack(payload)
			if err != nil {
				log.WithError(err).Error("failed to marshal process event delivery payload")
				failedInBatch++
				continue
			}

			taskName := convoy.EventProcessor
			job := &queue.Job{
				ID:      delivery.UID,
				Payload: data,
				Delay:   1 * time.Second,
			}
			err = q.Write(taskName, convoy.EventQueue, job)
			if err != nil {
				log.WithError(err).Errorf("batch %d: failed to send event delivery %s to the queue", batchCount, delivery.UID)
				failedInBatch++
				continue
			}
			log.Infof("successfully re-queued delivery with id: %s", delivery.UID)
			processedInBatch++
		}

		// Update tracking counters in Redis only (atomic operations)
		if tracker != nil && batchID != "" {
			batchTotal := processedInBatch + failedInBatch

			// Increment total count for this batch (what we're actually processing)
			if batchTotal > 0 {
				if err := tracker.IncrementTotal(ctx, batchID, batchTotal); err != nil {
					log.WithError(err).Error("Failed to increment total count")
				}
			}

			// Increment processed count atomically in Redis
			if processedInBatch > 0 {
				if err := tracker.IncrementProcessed(ctx, batchID, processedInBatch); err != nil {
					log.WithError(err).Error("Failed to increment processed count")
				}
			}
			if failedInBatch > 0 {
				if err := tracker.IncrementFailed(ctx, batchID, failedInBatch); err != nil {
					log.WithError(err).Error("Failed to increment failed count")
				}
			}
		}

		log.Infof("batch %d: successfully re-queued %d deliveries", batchCount, len(batch))
		batchCount++
	}
}
