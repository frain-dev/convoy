package bolt

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/frain-dev/convoy/datastore"
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

func NewEventDeliveryRepository(db *bbolt.DB) datastore.EventDeliveryRepository {
	return &eventDeliveryRepo{db: db}
}

func (e *eventDeliveryRepo) CreateEventDelivery(ctx context.Context, delivery *datastore.EventDelivery) error {
	return createUpdateEventDelivery(e.db, delivery)
}

func createUpdateEventDelivery(db *bbolt.DB, delivery *datastore.EventDelivery) error {
	return db.Update(func(tx *bbolt.Tx) error {
		b := getSubBucket(tx, eventDeliveryBucketName)

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

func (e *eventDeliveryRepo) FindEventDeliveryByID(ctx context.Context, uid string) (*datastore.EventDelivery, error) {
	var delivery datastore.EventDelivery
	err := e.db.View(func(tx *bbolt.Tx) error {

		buf := getSubBucket(tx, eventDeliveryBucketName).Get([]byte(uid))
		if buf == nil {
			return fmt.Errorf("event delivery with id (%s) does not exist", uid)
		}

		err := json.Unmarshal(buf, &delivery)
		if err != nil {
			return err
		}

		return nil
	})

	return &delivery, err
}

func (e *eventDeliveryRepo) FindEventDeliveriesByIDs(ctx context.Context, uids []string) ([]datastore.EventDelivery, error) {
	deliveries := make([]datastore.EventDelivery, len(uids))

	err := e.db.View(func(tx *bbolt.Tx) error {
		for i, uid := range uids {
			var delivery datastore.EventDelivery
			buf := getSubBucket(tx, eventDeliveryBucketName).Get([]byte(uid))
			if buf == nil {
				log.Errorf("event delivery with id (%s) does not exist", uid)
				continue
			}

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

func (e *eventDeliveryRepo) FindEventDeliveriesByEventID(ctx context.Context, eventID string) ([]datastore.EventDelivery, error) {
	var deliveries []datastore.EventDelivery

	type eid struct {
		EventMetadata struct {
			UID string `json:"uid"`
		} `json:"event_metadata"`
	}

	err := e.db.View(func(tx *bbolt.Tx) error {

		var eid eid
		c := getSubBucket(tx, eventDeliveryBucketName).Cursor()

		var deliverySlice [][]byte

		// seek all event deliveries
		for k, v := c.First(); k != nil; k, v = c.Next() {
			fmt.Printf("key=%s, value=%s\n", k, v)

			err := json.Unmarshal(v, &eid)
			if err != nil {
				return err
			}

			if eid.EventMetadata.UID != eventID {
				continue
			}

			deliverySlice = append(deliverySlice, v)
		}

		deliveries = make([]datastore.EventDelivery, len(deliverySlice))
		for i, buf := range deliverySlice {
			var delivery datastore.EventDelivery
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

func (e *eventDeliveryRepo) UpdateStatusOfEventDelivery(ctx context.Context, delivery datastore.EventDelivery, status datastore.EventDeliveryStatus) error {
	delivery.Status = status
	delivery.UpdatedAt = primitive.NewDateTimeFromTime(time.Now())

	return createUpdateEventDelivery(e.db, &delivery)
}

func (e *eventDeliveryRepo) UpdateEventDeliveryWithAttempt(ctx context.Context, delivery datastore.EventDelivery, attempt datastore.DeliveryAttempt) error {
	delivery.DeliveryAttempts = append(delivery.DeliveryAttempts, attempt)

	return createUpdateEventDelivery(e.db, &delivery)
}

func (e *eventDeliveryRepo) LoadEventDeliveriesPaged(ctx context.Context, groupID, appID, eventID string, status []datastore.EventDeliveryStatus, searchParams models.SearchParams, pageable models.Pageable) ([]datastore.EventDelivery, models.PaginationData, error) {
	hasAppFilter := !util.IsStringEmpty(appID)
	hasGroupFilter := !util.IsStringEmpty(groupID)
	hasEventFilter := !util.IsStringEmpty(eventID)
	hasStatusFilter := len(status) > 0
	hasDateFilter := searchParams.CreatedAtEnd > 0 || searchParams.CreatedAtStart > 0

	if pageable.Page <= 0 {
		pageable.Page = 1
	}
	if pageable.PerPage <= 0 {
		pageable.PerPage = 1
	}

	prevPage := pageable.Page - 1
	lowerBound := pageable.PerPage * prevPage
	upperBound := pageable.PerPage * pageable.Page

	var deliveries []datastore.EventDelivery
	var pg models.PaginationData
	err := e.db.View(func(tx *bbolt.Tx) error {

		b := getSubBucket(tx, eventDeliveryBucketName)
		c := b.Cursor()

		i := 0
		// seek all event deliveries
		for k, v := c.First(); k != nil; k, v = c.Next() {
			if i >= lowerBound && i < upperBound {
				fmt.Printf("key=%s, value=%s\n", k, v)

				var d datastore.EventDelivery
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
				deliveries = append(deliveries, d)
			}
			i++
			if i == (pageable.PerPage*pageable.Page)+pageable.PerPage {
				break
			}
		}

		pg = models.PaginationData{
			Total:     int64(b.Stats().KeyN),
			Page:      int64(pageable.Page),
			PerPage:   int64(pageable.PerPage),
			Prev:      int64(prevPage),
			Next:      int64(pageable.Page + 1),
			TotalPage: int64(math.Ceil(float64(b.Stats().KeyN) / float64(pageable.PerPage))),
		}
		return nil
	})

	return deliveries, pg, err
}

func getSubBucket(tx *bbolt.Tx, subBucketName string) *bbolt.Bucket {
	return tx.Bucket([]byte(bucketName)).Bucket([]byte(subBucketName))
}
