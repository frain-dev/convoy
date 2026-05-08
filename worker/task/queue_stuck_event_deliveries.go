package task

import (
	"context"
	"fmt"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/queue/redis"
)

func QueueStuckEventDeliveries(ctx context.Context, edRepo datastore.EventDeliveryRepository, q queue.Queuer, logger log.Logger) {
	for {
		evs, err := edRepo.FindStuckEventDeliveriesByStatus(context.Background(), datastore.ScheduledEventStatus)
		if err != nil {
			logger.ErrorContext(ctx, fmt.Sprintf("an error occurred fetching stuck event deliveries: %v", err))
			continue
		}

		ids := func() []string {
			arr := make([]string, 0, len(evs))
			for i := 0; i < len(evs); i++ {
				arr = append(arr, evs[i].UID)
			}
			return arr
		}()

		err = q.(*redis.RedisQueue).DeleteEventDeliveriesFromQueue(convoy.EventQueue, ids)
		if err != nil {
			logger.ErrorContext(ctx, "an error occurred removing task with id from the queue", "error", err)
		}

		for i := 0; i < len(evs); i++ {
			eventDelivery := evs[i]

			payload := EventDelivery{
				EventDeliveryID: eventDelivery.UID,
				ProjectID:       eventDelivery.ProjectID,
			}

			data, err := msgpack.EncodeMsgPack(payload)
			if err != nil {
				logger.ErrorContext(ctx, fmt.Sprintf("an error occurred encoding stuck event delivery with id %s: %v", eventDelivery.UID, err))
				continue
			}

			job := &queue.Job{
				ID:      eventDelivery.UID,
				Payload: data,
				Delay:   1 * time.Second,
			}

			err = q.Write(ctx, convoy.EventProcessor, convoy.EventQueue, job)
			if err != nil {
				logger.ErrorContext(ctx, fmt.Sprintf("an error occurred queueing stuck event delivery with id %s: %v", eventDelivery.UID, err))
				continue
			}
		}

		time.Sleep(time.Minute)
	}
}
