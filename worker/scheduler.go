package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/limiter"
	"github.com/frain-dev/convoy/queue"
	redisqueue "github.com/frain-dev/convoy/queue/redis"
	"github.com/frain-dev/convoy/worker/task"
	"github.com/frain-dev/disq"
	log "github.com/sirupsen/logrus"
)

func RegisterNewGroupTask(applicationRepo datastore.ApplicationRepository, eventDeliveryRepo datastore.EventDeliveryRepository, groupRepo datastore.GroupRepository, rateLimiter limiter.RateLimiter, eventRepo datastore.EventRepository, cache cache.Cache, eventQueue queue.Queuer) {
	go func() {
		for {
			filter := &datastore.GroupFilter{}
			groups, err := groupRepo.LoadGroups(context.Background(), filter)
			if err != nil {
				log.WithError(err).Error("failed to load groups")
			}
			for _, g := range groups {
				pEvtDelTask := convoy.EventProcessor.SetPrefix(g.Name)
				pEvtCrtTask := convoy.CreateEventProcessor.SetPrefix(g.Name)

				t, _ := disq.Tasks.LoadTask(string(pEvtCrtTask))
				if t == nil {
					s, _ := disq.Tasks.LoadTask(string(pEvtDelTask))
					if s == nil {
						handler := task.ProcessEventDelivery(applicationRepo, eventDeliveryRepo, groupRepo, rateLimiter)
						log.Infof("Registering event delivery task handler for %s", g.Name)
						task.CreateTask(pEvtDelTask, *g, handler)

						eventCreatedhandler := task.ProcessEventCreated(applicationRepo, eventRepo, groupRepo, eventDeliveryRepo, cache, eventQueue)
						log.Infof("Registering event creation task handler for %s", g.Name)
						task.CreateTask(pEvtCrtTask, *g, eventCreatedhandler)
					}
				}
			}
		}
	}()
}

func RequeueEventDeliveries(status string, timeInterval string, eventDeliveryRepo datastore.EventDeliveryRepository, groupRepo datastore.GroupRepository, eventQueue queue.Queuer) error {
	d, err := time.ParseDuration(timeInterval)
	if err != nil {
		return fmt.Errorf("failed to parse time duration")
	}

	s := datastore.EventDeliveryStatus(status)
	if !s.IsValid() {
		return fmt.Errorf("invalid event delivery status %s", s)
	}
	log.Infof("Requeuing for Status %v", status)

	now := time.Now()
	then := now.Add(-d)
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
		return fmt.Errorf("invalid queue type for requeing event deliveries: %T", eventQueue)
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go ProcessEventDeliveryBatches(ctx, s, eventDeliveryRepo, groupRepo, deliveryChan, q, &wg)

	counter, err := eventDeliveryRepo.CountDeliveriesByStatus(ctx, s, searchParams)
	if err != nil {
		return fmt.Errorf("failed to count event deliveries")
	}
	log.Infof("total number of event deliveries to requeue is %d", counter)

	for {
		deliveries, _, err := eventDeliveryRepo.LoadEventDeliveriesPaged(ctx, "", "", "", []datastore.EventDeliveryStatus{s}, searchParams, pageable)
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
	return nil
}

func ProcessEventDeliveryBatches(ctx context.Context, status datastore.EventDeliveryStatus, eventDeliveryRepo datastore.EventDeliveryRepository, groupRepo datastore.GroupRepository, deliveryChan <-chan []datastore.EventDelivery, q *redisqueue.RedisQueue, wg *sync.WaitGroup) {
	defer wg.Done()

	// groups serves as a cache for already fetched groups
	groups := map[string]*datastore.Group{}

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

		// remove these event deliveries from the zset
		err := q.DeleteEventDeliveriesFromZSET(ctx, batchIDs)
		if err != nil {
			log.WithError(err).WithField("ids", batchIDs).Errorf("batch %d: failed to delete event deliveries from zset", batchCount)
		}

		// // remove these event deliveries from the stream
		err = q.DeleteEventDeliveriesFromStream(ctx, batchIDs)
		if err != nil {
			log.WithError(err).WithField("ids", batchIDs).Errorf("batch %d: failed to delete event deliveries from stream", batchCount)
		}

		var group *datastore.Group
		for i := range batch {
			delivery := &batch[i]
			groupID := delivery.AppMetadata.GroupID

			group, ok = groups[groupID]
			if !ok { // never seen this group before, so fetch and cache
				group, err = groupRepo.FetchGroupByID(ctx, delivery.AppMetadata.GroupID)
				if err != nil {
					log.WithError(err).Errorf("batch %d: failed to fetch group %s for delivery %s", batchCount, delivery.AppMetadata.GroupID, delivery.UID)
					continue
				}
				groups[groupID] = group
			}

			taskName := convoy.EventProcessor.SetPrefix(group.Name)
			job := &queue.Job{
				ID: delivery.UID,
			}
			err = q.Publish(ctx, taskName, job, 1*time.Second)
			if err != nil {
				log.WithError(err).Errorf("batch %d: failed to send event delivery %s to the queue", batchCount, delivery.ID)
			}
			log.Infof("sucessfully requeued delivery with id: %s", delivery.UID)
		}

		log.Infof("batch %d: sucessfully requeued %d deliveries", batchCount, len(batch))
		batchCount++
	}
}
