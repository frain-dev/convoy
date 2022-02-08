package main

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/frain-dev/convoy"

	redisqueue "github.com/frain-dev/convoy/queue/redis"

	log "github.com/sirupsen/logrus"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/frain-dev/convoy/datastore"

	"github.com/spf13/cobra"
)

func addRetryCommand(a *app) *cobra.Command {
	var status string
	var timeInterval string

	cmd := &cobra.Command{
		Use:   "retry",
		Short: "retry event deliveries with a particular status in a timeframe",
		Run: func(cmd *cobra.Command, args []string) {

			d, err := time.ParseDuration(timeInterval)
			if err != nil {
				log.WithError(err).Fatal("failed to parse time duration")
			}

			now := time.Now()
			then := now.Add(-d)

			s := datastore.EventDeliveryStatus(status)
			searchParams := datastore.SearchParams{
				CreatedAtStart: int64(primitive.NewDateTimeFromTime(then)),
				CreatedAtEnd:   int64(primitive.NewDateTimeFromTime(now)),
			}

			pageable := datastore.Pageable{
				Page:    0,
				PerPage: 1000,
				Sort:    -1,
			}

			deliveryChan := make(chan []datastore.EventDelivery, 1)

			count := 0

			ctx := context.Background()
			var q *redisqueue.RedisQueue

			switch a.eventQueue.(type) {
			case *redisqueue.RedisQueue:
				q = a.eventQueue.(*redisqueue.RedisQueue)
			default:
				log.WithError(err).Fatalf("the retry command only works with redis queue for now")
			}

			var wg sync.WaitGroup

			wg.Add(1)
			go processEventDeliveryBatches(ctx, a, deliveryChan, q, &wg)

			for {
				deliveries, paginationData, err := a.eventDeliveryRepo.LoadEventDeliveriesPaged(ctx, "", "", "", []datastore.EventDeliveryStatus{s}, searchParams, pageable)
				if err != nil {
					log.WithError(err).Errorf("successfully fetched %d event deliveries, encountered error fetching page %d", count, pageable.Page)
					close(deliveryChan)
					break
				}

				count += len(deliveries)
				deliveryChan <- deliveries
				pageable.Page = int(paginationData.Next)
			}

			wg.Wait()
			os.Exit(0)
		},
	}

	cmd.Flags().StringVar(&status, "status", "", "Log time interval")
	cmd.Flags().StringVar(&timeInterval, "time", "", " time interval")
	return cmd
}

func processEventDeliveryBatches(ctx context.Context, a *app, deliveryChan <-chan []datastore.EventDelivery, q *redisqueue.RedisQueue, wg *sync.WaitGroup) {
	defer wg.Done()

	groups := map[string]*datastore.Group{}

	batchCount := 1
	for {
		batch := <-deliveryChan

		// the channel has been closed and there are no more deliveries coming in
		if batch == nil {
			return
		}

		batchIDs := make([]string, len(batch))
		for i := range batch {
			batchIDs[i] = batch[i].UID
		}

		err := q.DeleteEventDeliveriesFromZSET(ctx, batchIDs)
		if err != nil {
			log.WithError(err).WithField("ids", batchIDs).Error("failed to delete event deliveries from zset")
			// put continue here? @all reviewers
		}

		err = q.DeleteEventDeliveriesFromStream(ctx, batchIDs)
		if err != nil {
			log.WithError(err).WithField("ids", batchIDs).Error("failed to delete event deliveries from stream")
			// put continue here? @all reviewers
		}

		var group *datastore.Group
		var ok bool
		for i := range batch {
			delivery := &batch[i]
			groupID := delivery.AppMetadata.GroupID

			group, ok = groups[groupID]
			if !ok {
				group, err = a.groupRepo.FetchGroupByID(ctx, delivery.AppMetadata.GroupID)
				if err != nil {
					log.WithError(err).Errorf("failed to fetch group %s for delivery %s", delivery.AppMetadata.GroupID, delivery.UID)
					continue
				}
				groups[groupID] = group
			}

			taskName := convoy.EventProcessor.SetPrefix(group.Name)
			err = q.Write(ctx, taskName, delivery, 1*time.Second)
			if err != nil {
				log.WithError(err).Errorf("failed to send event delivery %s to the queue", delivery.ID)
			}
		}

		log.WithField("ids", batchIDs).Infof("sucessfully requeued %d deliveries in batch %d", len(batch), batchCount)
		batchCount++
	}
}
