package bolt

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	log "github.com/sirupsen/logrus"
	"go.etcd.io/bbolt"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const eventDeliveryBucketName = "eventdeliveries"

type eventDeliveryRepo struct {
	db *bbolt.DB
}

func NewEventDeliveryRepo(db *bbolt.DB) convoy.EventDeliveryRepository {
	return &eventDeliveryRepo{db: db}
}

func (e *eventDeliveryRepo) CreateEventDelivery(ctx context.Context, delivery *convoy.EventDelivery) error {
	return createUpdateEventDelivery(e.db, delivery)
}

func createUpdateEventDelivery(db *bbolt.DB, delivery *convoy.EventDelivery) error {
	return db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(eventDeliveryBucketName))

		buf, err := json.Marshal(delivery)
		if err != nil {
			return err
		}

		err = b.Put([]byte(delivery.UID), buf)
		if err != nil {
			return err
		}

		return nil
	})
}

func (e *eventDeliveryRepo) FindEventDeliveryByID(ctx context.Context, uid string) (*convoy.EventDelivery, error) {
	var delivery *convoy.EventDelivery
	err := e.db.View(func(tx *bbolt.Tx) error {

		buf := tx.Bucket([]byte(eventDeliveryBucketName)).Get([]byte(uid))
		if buf == nil {
			return fmt.Errorf("event delivery with id (%s) does not exist", uid)
		}

		err := json.Unmarshal(buf, delivery)
		if err != nil {
			return err
		}

		return nil
	})

	return delivery, err
}

func (e *eventDeliveryRepo) FindEventDeliveriesByIDs(ctx context.Context, uids []string) ([]convoy.EventDelivery, error) {
	deliveries := make([]convoy.EventDelivery, len(uids))

	err := e.db.View(func(tx *bbolt.Tx) error {

		for i, uid := range uids {
			var delivery *convoy.EventDelivery
			buf := tx.Bucket([]byte(eventDeliveryBucketName)).Get([]byte(uid))
			if buf == nil {
				log.Errorf("event delivery with id (%s) does not exist", uid)
				continue
			}

			err := json.Unmarshal(buf, delivery)
			if err != nil {
				return err
			}

			deliveries[i] = *delivery
		}
		return nil
	})

	return deliveries, err
}

func (e *eventDeliveryRepo) FindEventDeliveriesByEventID(ctx context.Context, eventID string) ([]convoy.EventDelivery, error) {
	var deliveries []convoy.EventDelivery

	type eid struct {
		EventID string `json:"event_id"`
	}

	err := e.db.View(func(tx *bbolt.Tx) error {

		var eid eid
		c := tx.Bucket([]byte(eventDeliveryBucketName)).Cursor()

		var deliverySlice [][]byte

		// seek all event deliveries
		for k, v := c.First(); k != nil; k, v = c.Next() {
			fmt.Printf("key=%s, value=%s\n", k, v)

			err := json.Unmarshal(v, &eid)
			if err != nil {
				return err
			}

			if eid.EventID != eventID {
				continue
			}

			deliverySlice = append(deliverySlice, v)
		}

		deliveries = make([]convoy.EventDelivery, len(deliverySlice))
		for i, buf := range deliverySlice {
			var delivery convoy.EventDelivery
			err := json.Unmarshal(buf, &delivery)
			if err != nil {
				return err
			}

			deliveries[i] = delivery
		}

		return nil
	})

	return deliveries, err
}

func (e *eventDeliveryRepo) UpdateStatusOfEventDelivery(ctx context.Context, delivery convoy.EventDelivery, status convoy.EventDeliveryStatus) error {
	delivery.Status = status
	delivery.UpdatedAt = primitive.NewDateTimeFromTime(time.Now())

	return createUpdateEventDelivery(e.db, &delivery)
}

func (e eventDeliveryRepo) UpdateEventDeliveryWithAttempt(ctx context.Context, delivery convoy.EventDelivery, attempt convoy.DeliveryAttempt) error {
	delivery.DeliveryAttempts = append(delivery.DeliveryAttempts, attempt)

	return createUpdateEventDelivery(e.db, &delivery)
}

func (e *eventDeliveryRepo) LoadEventDeliveriesPaged(ctx context.Context, groupID, appID, eventID string, status []convoy.EventDeliveryStatus, searchParams models.SearchParams, pageable models.Pageable) ([]convoy.EventDelivery, models.PaginationData, error) {
	hasAppFilter := !util.IsStringEmpty(appID)
	hasGroupFilter := !util.IsStringEmpty(groupID)
	hasEventFilter := !util.IsStringEmpty(eventID)
	hasStatusFilter := len(status) > 0
	hasDateFilter := searchParams.CreatedAtEnd > 0 || searchParams.CreatedAtStart > 0

	var deliveries []convoy.EventDelivery

	err := e.db.View(func(tx *bbolt.Tx) error {

		c := tx.Bucket([]byte(eventDeliveryBucketName)).Cursor()

		i := 0
		// seek all event deliveries
		for k, v := c.First(); k != nil; k, v = c.Next() {
			fmt.Printf("key=%s, value=%s\n", k, v)

			var d convoy.EventDelivery
			err := json.Unmarshal(v, &d)
			if err != nil {
				return err
			}

			if hasAppFilter && d.AppMetadata.UID != appID {
				continue
			}

			if hasGroupFilter && d.AppMetadata.GroupID != groupID {
				continue
			}

			if hasEventFilter && d.EventMetadata.UID != eventID {
				continue
			}

			if hasStatusFilter {
				found := false
				for _, deliveryStatus := range status {
					if d.Status == deliveryStatus {
						found = true
					}
				}

				if !found {
					continue
				}
			}

			if hasDateFilter {
				createdEnd := primitive.NewDateTimeFromTime(time.Unix(searchParams.CreatedAtEnd, 0))
				createdStart := primitive.NewDateTimeFromTime(time.Unix(searchParams.CreatedAtStart, 0))

				ok := false
				if d.CreatedAt <= createdEnd {
					ok = true
				}

				if d.CreatedAt >= createdStart {
					ok = true
				}

				if !ok {
					continue
				}
			}

			deliveries[i] = d
			i++
			if i == pageable.PerPage {
				break
			}
		}

		return nil
	})

	return deliveries, models.PaginationData{PerPage: int64(pageable.PerPage)}, err
}
