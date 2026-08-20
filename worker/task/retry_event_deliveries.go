package task

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/event_deliveries"
	"github.com/frain-dev/convoy/internal/pkg/batch_tracker"
	"github.com/frain-dev/convoy/internal/projects"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/util"
)

// RetryEventDeliveries requeues deliveries for one project, or for the whole
// instance when projectID is empty. Instance-wide requeue is reserved for the
// admin surfaces (instance-admin API, explicitly confirmed CLI).
func RetryEventDeliveries(logger log.Logger, db database.Database, eventQueue queue.Queuer, projectID string, statuses []datastore.EventDeliveryStatus, lookBackDuration, eventId string) {
	RetryEventDeliveriesWithTracker(logger, db, eventQueue, projectID, statuses, lookBackDuration, eventId, "", nil)
}

func RetryEventDeliveriesWithTracker(logger log.Logger, db database.Database, eventQueue queue.Queuer, projectID string, statuses []datastore.EventDeliveryStatus, lookBackDuration, eventId, batchID string, tracker batch_tracker.Tracker) {
	if len(statuses) == 1 && util.IsStringEmpty(string(statuses[0])) {
		statuses = []datastore.EventDeliveryStatus{"Retry", "Scheduled", "Processing"}
	}

	if util.IsStringEmpty(lookBackDuration) {
		// TODO(subomi): Setup configuration
		lookBackDuration = "5h"
	}

	d, err := time.ParseDuration(lookBackDuration)
	if err != nil {
		logger.Error("Failed to parse time duration")
	}
	now := time.Now()
	then := now.Add(-d)

	ctx := context.Background()

	// LoadEventDeliveriesPaged requires a real project_id (equality, not the
	// old empty-string wildcard) so the dashboard list can range-scan.
	// Instance-admin retry still passes "" for the whole instance; resolve
	// that to every live project and page each one.
	projectIDs, err := retryProjectIDs(ctx, logger, db, projectID)
	if err != nil {
		logger.Error("Failed to resolve projects for event delivery retry", "error", err)
		return
	}

	// Initialize repositories and queue once
	eventDeliveryRepo := event_deliveries.New(logger, db)

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
			logger.Error("Failed to create batch tracker", "error", err)
		}
	}

	for _, status := range statuses {
		allStatusesWg.Add(1)
		go func(s datastore.EventDeliveryStatus) {
			defer allStatusesWg.Done()
			logger.Info(fmt.Sprintf("Searching for events with status %s", s))
			searchParams := datastore.SearchParams{
				CreatedAtStart: then.Unix(),
				CreatedAtEnd:   now.Unix(),
			}

			deliveryChan := make(chan []datastore.EventDelivery, 4)
			count := 0

			var wg sync.WaitGroup

			wg.Add(1)

			go processEventDeliveryBatch(ctx, projectID, s, eventDeliveryRepo, deliveryChan, eventQueue, &wg, batchID, tracker, logger)

			counter, err := eventDeliveryRepo.CountDeliveriesByStatus(ctx, projectID, s, searchParams)
			if err != nil {
				logger.Error("Failed to count event deliveries")
			}
			logger.Info(fmt.Sprintf("Total number of event deliveries to requeue is %d", counter))

			fetchErr := false
			for _, pid := range projectIDs {
				pageable := datastore.Pageable{
					Direction:  datastore.Next,
					PerPage:    1000,
					NextCursor: datastore.DefaultCursor,
				}
				for {
					deliveries, pagination, err := eventDeliveryRepo.LoadEventDeliveriesPaged(ctx, pid, []string{}, eventId, "", []datastore.EventDeliveryStatus{s}, searchParams, pageable, "", "", "")
					if err != nil {
						logger.Error(fmt.Sprintf("successfully fetched %d event deliveries but with error: %v", count, err))
						fetchErr = true
						break
					}

					if len(deliveries) == 0 {
						break
					}

					count += len(deliveries)
					deliveryChan <- deliveries
					pageable.NextCursor = pagination.NextPageCursor
				}
				if fetchErr {
					break
				}
			}
			if count == 0 && !fetchErr {
				logger.Warn("no deliveries received from db, exiting")
			}
			close(deliveryChan)
			logger.Info("closed delivery channel")

			logger.Info("waiting for batch processor to finish")
			wg.Wait()
		}(status)
	}

	// Wait for all status processing to complete
	allStatusesWg.Wait()

	// Complete batch tracking if tracker is provided (after all statuses are processed)
	if tracker != nil && batchID != "" {
		if err := tracker.SyncCounters(ctx, batchID); err != nil {
			logger.Error("Failed to sync batch counters", "error", err)
		}
		if err := tracker.CompleteBatch(ctx, batchID); err != nil {
			logger.Error("Failed to complete batch tracking", "error", err)
		}
	}
}

// retryProjectIDs returns the single project when scoped, or every live
// project when projectID is empty (instance-admin / --all-projects).
func retryProjectIDs(ctx context.Context, logger log.Logger, db database.Database, projectID string) ([]string, error) {
	if !util.IsStringEmpty(projectID) {
		return []string{projectID}, nil
	}
	listed, err := projects.New(logger, db).LoadProjects(ctx, &datastore.ProjectFilter{})
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(listed))
	for _, p := range listed {
		if p == nil || util.IsStringEmpty(p.UID) {
			continue
		}
		ids = append(ids, p.UID)
	}
	return ids, nil
}

func processEventDeliveryBatch(ctx context.Context, projectID string, s datastore.EventDeliveryStatus, edRepo datastore.EventDeliveryRepository, deliveryChan <-chan []datastore.EventDelivery, q queue.Queuer, wg *sync.WaitGroup, batchID string, t batch_tracker.Tracker, l log.Logger) {
	defer wg.Done()

	batchCount := 1
	for {
		// ok will return false if the channel is closed and drained(empty), at which point
		// we should return
		batch, ok := <-deliveryChan
		if !ok {
			// the channel has been closed and there are no more deliveries coming in
			l.Warn("batch processor exiting")
			return
		}

		batchIDs := make([]string, len(batch))
		for i := range batch {
			batchIDs[i] = batch[i].UID
		}

		if s == datastore.ProcessingEventStatus {
			err := edRepo.UpdateStatusOfEventDeliveries(ctx, projectID, batchIDs, datastore.ScheduledEventStatus)
			if err != nil {
				l.Error(fmt.Sprintf("batch %d: failed to update event deliveries status: %v", batchCount, err))
			}
		}

		// remove these event deliveries queue
		err := removeQueuedJobs(q, convoy.EventQueue, batchIDs)
		if err != nil {
			l.Error(fmt.Sprintf("batch %d: failed to delete event deliveries from zset", batchCount), "error", err, "ids", batchIDs)
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
				l.Error("failed to marshal process event delivery payload", "error", err)
				failedInBatch++
				continue
			}

			taskName := convoy.EventProcessor
			job := &queue.Job{
				ID:      delivery.UID,
				Payload: data,
				Delay:   1 * time.Second,
			}
			err = q.Write(ctx, taskName, convoy.EventQueue, job)
			if err != nil {
				l.Error(fmt.Sprintf("batch %d: failed to send event delivery %s to the queue: %v", batchCount, delivery.UID, err))
				failedInBatch++
				continue
			}
			l.Info(fmt.Sprintf("successfully re-queued delivery with id: %s", delivery.UID))
			processedInBatch++
		}

		// Update tracking counters in Redis only (atomic operations)
		if t != nil && batchID != "" {
			batchTotal := processedInBatch + failedInBatch

			// Increment total count for this batch (what we're actually processing)
			if batchTotal > 0 {
				if err := t.IncrementTotal(ctx, batchID, batchTotal); err != nil {
					l.Error("Failed to increment total count", "error", err)
				}
			}

			// Increment processed count atomically in Redis
			if processedInBatch > 0 {
				if err := t.IncrementProcessed(ctx, batchID, processedInBatch); err != nil {
					l.Error("Failed to increment processed count", "error", err)
				}
			}
			if failedInBatch > 0 {
				if err := t.IncrementFailed(ctx, batchID, failedInBatch); err != nil {
					l.Error("Failed to increment failed count", "error", err)
				}
			}
		}

		l.Info(fmt.Sprintf("batch %d: successfully re-queued %d deliveries", batchCount, len(batch)))
		batchCount++
	}
}
