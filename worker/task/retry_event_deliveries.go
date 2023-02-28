package task

import (
	"context"
	"encoding/json"
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

func RetryEventDeliveries(statuses []datastore.EventDeliveryStatus, lookBackDuration string, db database.Database, eventQueue queue.Queuer) {
	if statuses == nil {
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
		searchParams := datastore.SearchParams{
			CreatedAtStart: then.Unix(),
			CreatedAtEnd:   now.Unix(),
		}

		pageable := datastore.Pageable{
			Page:    0,
			PerPage: 1000,
			Sort:    -1,
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
		eventDeliveryRepo := postgres.NewEventDeliveryRepo(db)
		projectRepo := postgres.NewProjectRepo(db)

		go processEventDeliveryBatch(ctx, status, eventDeliveryRepo, projectRepo, deliveryChan, q, &wg)

		counter, err := eventDeliveryRepo.CountDeliveriesByStatus(ctx, status, searchParams)
		if err != nil {
			log.Error("Failed to count event deliveries")
		}
		log.Infof("Total number of event deliveries to requeue is %d", counter)

		for {
			deliveries, _, err := eventDeliveryRepo.LoadEventDeliveriesPaged(ctx, "", []string{}, "", []datastore.EventDeliveryStatus{status}, searchParams, pageable)
			if err != nil {
				log.WithError(err).Errorf("successfully fetched %d event deliveries, encountered error fetching page %d", count, pageable.Page)
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
			pageable.Page++
		}

		log.Info("waiting for batch processor to finish")
		wg.Wait()
	}
}

func processEventDeliveryBatch(ctx context.Context, status datastore.EventDeliveryStatus, eventDeliveryRepo datastore.EventDeliveryRepository, projectRepo datastore.ProjectRepository, deliveryChan <-chan []datastore.EventDelivery, q *redisqueue.RedisQueue, wg *sync.WaitGroup) {
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
			err := eventDeliveryRepo.UpdateStatusOfEventDeliveries(ctx, batchIDs, datastore.ScheduledEventStatus)
			if err != nil {
				log.WithError(err).Errorf("batch %d: failed to update event deliveries status", batchCount)
			}
		}

		// remove these event deliveries queue
		err := q.DeleteEventDeliveriesfromQueue(convoy.EventQueue, batchIDs)
		if err != nil {
			log.WithError(err).WithField("ids", batchIDs).Errorf("batch %d: failed to delete event deliveries from zset", batchCount)
		}

		for i := range batch {
			delivery := &batch[i]

			taskName := convoy.EventProcessor
			job := &queue.Job{
				ID:      delivery.UID,
				Payload: json.RawMessage(delivery.UID),
				Delay:   1 * time.Second,
			}
			err := q.Write(taskName, convoy.EventQueue, job)
			if err != nil {
				log.WithError(err).Errorf("batch %d: failed to send event delivery %s to the queue", batchCount, delivery.UID)
			}
			log.Infof("sucessfully requeued delivery with id: %s", delivery.UID)
		}

		log.Infof("batch %d: sucessfully requeued %d deliveries", batchCount, len(batch))
		batchCount++
	}
}
