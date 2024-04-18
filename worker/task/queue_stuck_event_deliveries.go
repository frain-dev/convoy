package task

import (
	"context"
	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/queue/redis"
	"time"
)

func QueueStuckEventDeliveries(ctx context.Context, ticker *time.Ticker, edRepo datastore.EventDeliveryRepository, q queue.Queuer) {
	for {
		select {
		case <-ticker.C:
			evs, err := edRepo.FindStuckEventDeliveriesByStatus(context.Background(), datastore.ScheduledEventStatus)
			if err != nil {
				log.FromContext(ctx).WithError(err).Errorf("an error occurred fetching stuck event deliveries")
				continue
			}

			ids := func() []string {
				arr := make([]string, len(evs))
				for i := 0; i < len(evs); i++ {
					arr = append(arr, evs[i].UID)
				}
				return arr
			}()

			err = q.(*redis.RedisQueue).DeleteEventDeliveriesFromQueue(convoy.EventQueue, ids)
			if err != nil {
				log.FromContext(ctx).WithError(err).Error("an error occurred removing task with id from the queue")
			}

			for i := 0; i < len(evs); i++ {
				eventDelivery := evs[i]

				payload := EventDelivery{
					EventDeliveryID: eventDelivery.UID,
					ProjectID:       eventDelivery.ProjectID,
				}

				data, err := msgpack.EncodeMsgPack(payload)
				if err != nil {
					log.FromContext(ctx).WithError(err).Errorf("an error occurred encoding stuck event delivery with id %s", eventDelivery.UID)
					continue
				}

				job := &queue.Job{
					ID:      eventDelivery.UID,
					Payload: data,
					Delay:   1 * time.Second,
				}

				err = q.Write(convoy.EventProcessor, convoy.EventQueue, job)
				if err != nil {
					log.FromContext(ctx).WithError(err).Errorf("an error occurred queueing stuck event delivery with id %s", eventDelivery.UID)
					continue
				}
			}
		default:
			continue
		}
	}
}
