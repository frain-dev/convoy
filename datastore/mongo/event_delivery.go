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
	store datastore.Store
}

const (
	EventDeliveryCollection = "eventdeliveries"
)

func NewEventDeliveryRepository(db *mongo.Database, store datastore.Store) datastore.EventDeliveryRepository {
	return &eventDeliveryRepo{
		inner: db.Collection(EventDeliveryCollection),
		store: store,
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

	filter := bson.M{"uid": id, "document_status": datastore.ActiveDocumentStatus}

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
		"document_status": datastore.ActiveDocumentStatus,
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

	filter := bson.M{"event_id": eventID, "document_status": datastore.ActiveDocumentStatus}

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

func (db *eventDeliveryRepo) CountDeliveriesByStatus(ctx context.Context,
	status datastore.EventDeliveryStatus, searchParams datastore.SearchParams) (int64, error) {

	filter := bson.M{
		"status":          status,
		"document_status": datastore.ActiveDocumentStatus,
		"created_at":      getCreatedDateFilter(searchParams),
	}

	count, err := db.inner.CountDocuments(ctx, filter, nil)
	if err != nil {
		return 0, err
	}

	return count, nil
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

func (db *eventDeliveryRepo) UpdateStatusOfEventDeliveries(ctx context.Context, ids []string, status datastore.EventDeliveryStatus) error {

	filter := bson.M{
		"uid": bson.M{
			"$in": ids,
		},
		"document_status": datastore.ActiveDocumentStatus,
	}

	update := bson.M{
		"$set": bson.M{
			"status":     status,
			"updated_at": primitive.NewDateTimeFromTime(time.Now()),
		},
	}
	_, err := db.inner.UpdateMany(ctx, filter, update)
	if err != nil {
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
	filter := getFilter(groupID, appID, eventID, status, searchParams)

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

func (db *eventDeliveryRepo) CountEventDeliveries(ctx context.Context, groupID, appID, eventID string, status []datastore.EventDeliveryStatus, searchParams datastore.SearchParams) (int64, error) {
	filter := getFilter(groupID, appID, eventID, status, searchParams)

	var count int64
	count, err := db.inner.CountDocuments(ctx, filter)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func getFilter(groupID string, appID string, eventID string, status []datastore.EventDeliveryStatus, searchParams datastore.SearchParams) bson.M {

	filter := bson.M{
		"document_status": datastore.ActiveDocumentStatus,
		"created_at":      getCreatedDateFilter(searchParams),
	}

	hasAppFilter := !util.IsStringEmpty(appID)
	hasGroupFilter := !util.IsStringEmpty(groupID)
	hasEventFilter := !util.IsStringEmpty(eventID)
	hasStatusFilter := len(status) > 0

	if hasAppFilter {
		filter["app_id"] = appID
	}

	if hasGroupFilter {
		filter["group_id"] = groupID
	}

	if hasEventFilter {
		filter["event_id"] = eventID
	}

	if hasStatusFilter {
		filter["status"] = bson.M{"$in": status}
	}

	return filter
}

func (db *eventDeliveryRepo) DeleteGroupEventDeliveries(ctx context.Context, filter *datastore.EventDeliveryFilter, hardDelete bool) error {
	update := bson.M{
		"deleted_at":      primitive.NewDateTimeFromTime(time.Now()),
		"document_status": datastore.DeletedDocumentStatus,
	}

	f := bson.M{
		"group_id":        filter.GroupID,
		"document_status": datastore.ActiveDocumentStatus,
		"created_at": bson.M{
			"$gte": primitive.NewDateTimeFromTime(time.Unix(filter.CreatedAtStart, 0)),
			"$lte": primitive.NewDateTimeFromTime(time.Unix(filter.CreatedAtEnd, 0)),
		},
	}

	err := db.store.DeleteMany(ctx, f, update, hardDelete)
	if err != nil {
		return err
	}
	return nil
}

func (db *eventDeliveryRepo) FindDiscardedEventDeliveries(ctx context.Context, appId, deviceId string, searchParams datastore.SearchParams) ([]datastore.EventDelivery, error) {
	filter := bson.M{
		"app_id":          appId,
		"device_id":       deviceId,
		"status":          datastore.DiscardedEventStatus,
		"created_at":      getCreatedDateFilter(searchParams),
		"document_status": datastore.ActiveDocumentStatus,
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

	if err := cur.Err(); err != nil {
		return nil, err
	}

	if err := cur.Close(ctx); err != nil {
		return deliveries, err
	}

	return deliveries, nil
}
