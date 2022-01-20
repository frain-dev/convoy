package mongo

import (
	"context"
	"errors"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	pager "github.com/gobeam/mongo-go-pagination"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type eventDeliveryRepo struct {
	inner *mongo.Collection
}

const (
	EventDeliveryCollection = "eventdeliveries"
)

func NewEventDeliveryRepository(db *mongo.Database) datastore.EventDeliveryRepository {
	return &eventDeliveryRepo{
		inner: db.Collection(EventDeliveryCollection),
	}
}

func (db *eventDeliveryRepo) CreateEventDelivery(ctx context.Context,
	eventDelivery *datastore.EventDelivery) error {

	eventDelivery.ID = primitive.NewObjectID()
	if util.IsStringEmpty(eventDelivery.UID) {
		eventDelivery.UID = uuid.New().String()
	}

	_, err := db.inner.InsertOne(ctx, eventDelivery)
	return err
}

func (db *eventDeliveryRepo) FindEventDeliveryByID(ctx context.Context,
	id string) (*datastore.EventDelivery, error) {
	e := new(datastore.EventDelivery)

	filter := bson.M{"uid": id, "document_status": bson.M{"$ne": datastore.DeletedDocumentStatus}}

	err := db.inner.FindOne(ctx, filter).Decode(&e)

	if errors.Is(err, mongo.ErrNoDocuments) {
		err = datastore.ErrEventDeliveryNotFound
	}

	return e, err
}

func (db *eventDeliveryRepo) FindEventDeliveriesByIDs(ctx context.Context,
	ids []string) ([]datastore.EventDelivery, error) {

	filter := bson.M{
		"uid": bson.M{
			"$in": ids,
		},
		"document_status": bson.M{
			"$ne": datastore.DeletedDocumentStatus,
		},
	}

	deliveries := make([]datastore.EventDelivery, 0)

	cur, err := db.inner.Find(ctx, filter, nil)
	if err != nil {
		return deliveries, err
	}

	for cur.Next(ctx) {
		var delivery datastore.EventDelivery
		if err := cur.Decode(&delivery); err != nil {
			return deliveries, err
		}

		deliveries = append(deliveries, delivery)
	}

	return deliveries, err
}

func (db *eventDeliveryRepo) FindEventDeliveriesByEventID(ctx context.Context,
	eventID string) ([]datastore.EventDelivery, error) {

	filter := bson.M{"event_id": eventID, "document_status": bson.M{"$ne": datastore.DeletedDocumentStatus}}

	deliveries := make([]datastore.EventDelivery, 0)

	cur, err := db.inner.Find(ctx, filter, nil)
	if err != nil {
		return deliveries, err
	}

	for cur.Next(ctx) {
		var delivery datastore.EventDelivery
		if err := cur.Decode(&delivery); err != nil {
			return deliveries, err
		}

		deliveries = append(deliveries, delivery)
	}

	if err := cur.Err(); err != nil {
		return nil, err
	}

	if err := cur.Close(ctx); err != nil {
		return deliveries, err
	}

	return deliveries, nil
}

func (db *eventDeliveryRepo) UpdateStatusOfEventDelivery(ctx context.Context,
	e datastore.EventDelivery, status datastore.EventDeliveryStatus) error {

	filter := bson.M{"uid": e.UID}
	update := bson.M{
		"$set": bson.M{
			"status":     status,
			"updated_at": primitive.NewDateTimeFromTime(time.Now()),
		},
	}

	result := db.inner.FindOneAndUpdate(ctx, filter, update)
	err := result.Err()
	if err != nil {
		log.WithError(err).Error("Failed to update event delivery status")
		return err
	}

	return nil
}

func (db *eventDeliveryRepo) UpdateEventDeliveryWithAttempt(ctx context.Context,
	e datastore.EventDelivery, attempt datastore.DeliveryAttempt) error {

	filter := bson.M{"uid": e.UID}
	update := bson.M{
		"$set": bson.M{
			"status":      e.Status,
			"description": e.Description,
			"metadata":    e.Metadata,
			"updated_at":  primitive.NewDateTimeFromTime(time.Now()),
		},
		"$push": bson.M{
			"attempts": attempt,
		},
	}

	_, err := db.inner.UpdateOne(ctx, filter, update)
	if err != nil {
		log.WithError(err).Errorf("error updating an event delivery %s - %s\n", e.UID, err.Error())
		return err
	}

	return nil
}

func (db *eventDeliveryRepo) LoadEventDeliveriesPaged(ctx context.Context, groupID, appID, eventID string, status []datastore.EventDeliveryStatus, searchParams datastore.SearchParams, pageable datastore.Pageable) ([]datastore.EventDelivery, datastore.PaginationData, error) {
	filter := bson.M{
		"document_status": bson.M{"$ne": datastore.DeletedDocumentStatus},
		"created_at":      getCreatedDateFilter(searchParams),
	}

	hasAppFilter := !util.IsStringEmpty(appID)
	hasGroupFilter := !util.IsStringEmpty(groupID)
	hasEventFilter := !util.IsStringEmpty(eventID)
	hasStatusFilter := len(status) > 0

	if hasAppFilter {
		filter["app_metadata.uid"] = appID
	}

	if hasGroupFilter {
		filter["app_metadata.group_id"] = groupID
	}

	if hasEventFilter {
		filter["event_metadata.uid"] = eventID
	}

	if hasStatusFilter {
		filter["status"] = bson.M{"$in": status}
	}

	var eventDeliveries []datastore.EventDelivery
	paginatedData, err := pager.New(db.inner).Context(ctx).Limit(int64(pageable.PerPage)).Page(int64(pageable.Page)).Sort("created_at", pageable.Sort).Filter(filter).Decode(&eventDeliveries).Find()
	if err != nil {
		return eventDeliveries, datastore.PaginationData{}, err
	}

	if eventDeliveries == nil {
		eventDeliveries = make([]datastore.EventDelivery, 0)
	}

	return eventDeliveries, datastore.PaginationData(paginatedData.Pagination), nil
}
