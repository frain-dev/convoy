package bolt

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	log "github.com/sirupsen/logrus"
	"go.etcd.io/bbolt"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type eventDeliveryRepo struct {
	bucketName string
	db         *bbolt.DB
}

func NewEventDeliveryRepository(db *bbolt.DB) datastore.EventDeliveryRepository {
	eventDeliveryBucketName := "eventdeliveries"
	err := db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(eventDeliveryBucketName))
		return err
	})

	if err != nil {
		return nil
	}

	return &eventDeliveryRepo{db: db, bucketName: eventDeliveryBucketName}
}

func (e *eventDeliveryRepo) CreateEventDelivery(ctx context.Context, delivery *datastore.EventDelivery) error {
	return e.createUpdateEventDelivery(delivery)
}

func (e *eventDeliveryRepo) createUpdateEventDelivery(delivery *datastore.EventDelivery) error {
	return e.db.Update(func(tx *bbolt.Tx) error {
		buf, err := json.Marshal(delivery)
		if err != nil {
			return err
		}

		b := tx.Bucket([]byte(e.bucketName))
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

		buf := tx.Bucket([]byte(e.bucketName)).Get([]byte(uid))
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
			buf := tx.Bucket([]byte(e.bucketName)).Get([]byte(uid))
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
		c := tx.Bucket([]byte(e.bucketName)).Cursor()

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

	return e.createUpdateEventDelivery(&delivery)
}

func (e *eventDeliveryRepo) UpdateEventDeliveryWithAttempt(ctx context.Context, delivery datastore.EventDelivery, attempt datastore.DeliveryAttempt) error {
	delivery.DeliveryAttempts = append(delivery.DeliveryAttempts, attempt)

	return e.createUpdateEventDelivery(&delivery)
}

func (e *eventDeliveryRepo) LoadEventDeliveriesPaged(ctx context.Context, groupID, appID, eventID string, status []datastore.EventDeliveryStatus, searchParams datastore.SearchParams, pageable datastore.Pageable) ([]datastore.EventDelivery, datastore.PaginationData, error) {
	f := &filter{
		groupID:      groupID,
		appID:        appID,
		eventID:      eventID,
		status:       status,
		searchParams: searchParams,

		hasAppFilter:       !util.IsStringEmpty(appID),
		hasGroupFilter:     !util.IsStringEmpty(groupID),
		hasEventFilter:     !util.IsStringEmpty(eventID),
		hasStatusFilter:    len(status) > 0,
		hasStartDateFilter: searchParams.CreatedAtStart > 0,
		hasEndDateFilter:   searchParams.CreatedAtEnd > 0,
	}

	if pageable.Page < 1 {
		pageable.Page = 1
	}
	if pageable.PerPage < 1 {
		pageable.PerPage = 10
	}

	prevPage := pageable.Page - 1
	lowerBound := pageable.PerPage * prevPage
	upperBound := pageable.PerPage * pageable.Page

	var deliveries []datastore.EventDelivery
	var pg datastore.PaginationData
	err := e.db.View(func(tx *bbolt.Tx) error {

		b := tx.Bucket([]byte(e.bucketName))
		c := b.Cursor()

		i := 0
		// seek all event deliveries
		for k, v := c.First(); k != nil; k, v = c.Next() {
			if i >= lowerBound && i < upperBound {
				var d datastore.EventDelivery
				err := json.Unmarshal(v, &d)
				if err != nil {
					return err
				}

				if !e.filterEventDelivery(f, &d) {
					continue
				}

				deliveries = append(deliveries, d)
			}
			i++
			if i == (pageable.PerPage*pageable.Page)+pageable.PerPage {
				break
			}
		}

		total, err := e.countEventDeliveriesWithFilter(f)
		if err != nil {
			return err
		}

		pg = datastore.PaginationData{
			Total:     total,
			Page:      int64(pageable.Page),
			PerPage:   int64(pageable.PerPage),
			Prev:      int64(prevPage),
			Next:      int64(pageable.Page + 1),
			TotalPage: int64(math.Ceil(float64(total) / float64(pageable.PerPage))),
		}
		return nil
	})

	return deliveries, pg, err
}

// countEventDeliveriesWithFilter counts all the event deliveries in the database that satisfy the filter
func (e *eventDeliveryRepo) countEventDeliveriesWithFilter(f *filter) (int64, error) {
	i := int64(0)
	err := e.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(e.bucketName))
		c := b.Cursor()

		// seek all event deliveries
		var d datastore.EventDelivery
		for k, v := c.First(); k != nil; k, v = c.Next() {
			err := json.Unmarshal(v, &d)
			if err != nil {
				return err
			}

			if e.filterEventDelivery(f, &d) {
				i++
			}
		}

		return nil
	})

	return i, err
}

type filter struct {
	groupID      string
	appID        string
	eventID      string
	status       []datastore.EventDeliveryStatus
	searchParams datastore.SearchParams

	hasAppFilter       bool
	hasGroupFilter     bool
	hasEventFilter     bool
	hasStatusFilter    bool
	hasStartDateFilter bool
	hasEndDateFilter   bool
}

func (e *eventDeliveryRepo) filterEventDelivery(f *filter, d *datastore.EventDelivery) bool {
	if f.hasAppFilter && d.AppMetadata.UID != f.appID {
		return false
	}

	if f.hasGroupFilter && d.AppMetadata.GroupID != f.groupID {
		return false
	}

	if f.hasEventFilter && d.EventMetadata.UID != f.eventID {
		return false
	}

	if f.hasStatusFilter {
		found := false
		for _, deliveryStatus := range f.status {
			if d.Status == deliveryStatus {
				found = true
			}
		}

		if !found {
			return false
		}
	}

	if f.hasStartDateFilter {
		createdStart := primitive.NewDateTimeFromTime(time.Unix(f.searchParams.CreatedAtStart, 0))
		ok := d.CreatedAt >= createdStart
		if !ok {
			return false
		}
	}

	if f.hasEndDateFilter {
		createdEnd := primitive.NewDateTimeFromTime(time.Unix(f.searchParams.CreatedAtEnd, 0))
		ok := d.CreatedAt <= createdEnd
		if !ok {
			return false
		}
	}

	return true
}
