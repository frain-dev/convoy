package task

import (
	"context"
	"encoding/json"
	"github.com/frain-dev/convoy/cache"
	"sync"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	redisqueue "github.com/frain-dev/convoy/queue/redis"
	"github.com/frain-dev/convoy/util"
)

func RetryEventDeliveries(db database.Database, cache cache.Cache, eventQueue queue.Queuer, statuses []datastore.EventDeliveryStatus, lookBackDuration string, eventId string) {
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

	for _, status := range statuses {
		log.Printf("Searching for events with status %s", status)
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

		ctx := context.Background()
		var q *redisqueue.RedisQueue
		q, ok := eventQueue.(*redisqueue.RedisQueue)
		if !ok {
			log.Errorf("Invalid queue type for requeing event deliveries: %T", eventQueue)
		}

		var wg sync.WaitGroup

		wg.Add(1)
		eventDeliveryRepo := postgres.NewEventDeliveryRepo(db, cache)

		go processEventDeliveryBatch(ctx, status, eventDeliveryRepo, deliveryChan, q, &wg)

		counter, err := eventDeliveryRepo.CountDeliveriesByStatus(ctx, "", status, searchParams)
		if err != nil {
			log.Error("Failed to count event deliveries")
		}
		log.Infof("Total number of event deliveries to requeue is %d", counter)

		for {
			deliveries, pagination, err := eventDeliveryRepo.LoadEventDeliveriesPaged(ctx, "", []string{}, eventId, "", []datastore.EventDeliveryStatus{status}, searchParams, pageable, "")
			if err != nil {
				log.WithError(err).Errorf("successfully fetched %d event deliveries", count)
				close(deliveryChan)
				log.Info("closed delivery channel")
				break
			}

			// stop when len(deliveries) is 0
			if len(deliveries) == 0 {
				log.Info("no deliveries received from db, exiting")
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
	}
}

func processEventDeliveryBatch(ctx context.Context, status datastore.EventDeliveryStatus, eventDeliveryRepo datastore.EventDeliveryRepository, deliveryChan <-chan []datastore.EventDelivery, q *redisqueue.RedisQueue, wg *sync.WaitGroup) {
	defer wg.Done()

	batchCount := 1
	for {
		// ok will return false if the channel is closed and drained(empty), at which point
		// we should return
		batch, ok := <-deliveryChan
		if !ok {
			// the channel has been closed and there are no more deliveries coming in
			log.Infof("batch processor exiting")
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

		for i := range batch {
			delivery := &batch[i]

			payload := EventDelivery{
				EventDeliveryID: delivery.UID,
				ProjectID:       delivery.ProjectID,
			}
			data, err := json.Marshal(payload)
			if err != nil {
				log.WithError(err).Error("failed to marshal process event delivery payload")
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
			}
			log.Infof("successfully re-queued delivery with id: %s", delivery.UID)
		}

		log.Infof("batch %d: successfully re-queued %d deliveries", batchCount, len(batch))
		batchCount++
	}
}
