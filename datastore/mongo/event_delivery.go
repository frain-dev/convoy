package mongo

import (
	"context"
	"errors"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type eventDeliveryRepo struct {
	store datastore.Store
}

func NewEventDeliveryRepository(store datastore.Store) datastore.EventDeliveryRepository {
	return &eventDeliveryRepo{
		store: store,
	}
}

func (db *eventDeliveryRepo) CreateEventDelivery(ctx context.Context,
	eventDelivery *datastore.EventDelivery,
) error {
	ctx = db.setCollectionInContext(ctx)

	eventDelivery.ID = primitive.NewObjectID()
	if util.IsStringEmpty(eventDelivery.UID) {
		eventDelivery.UID = uuid.New().String()
	}

	return db.store.Save(ctx, eventDelivery, nil)
}

func (db *eventDeliveryRepo) FindEventDeliveryByID(ctx context.Context,
	uid string,
) (*datastore.EventDelivery, error) {
	ctx = db.setCollectionInContext(ctx)

	eventDelivery := &datastore.EventDelivery{}

	err := db.store.FindByID(ctx, uid, nil, eventDelivery)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			err = datastore.ErrEventDeliveryNotFound
		}
		return nil, err
	}

	return eventDelivery, nil
}

func (db *eventDeliveryRepo) FindEventDeliveriesByIDs(ctx context.Context,
	ids []string,
) ([]datastore.EventDelivery, error) {
	ctx = db.setCollectionInContext(ctx)

	filter := bson.M{
		"uid": bson.M{
			"$in": ids,
		},
	}

	var deliveries []datastore.EventDelivery

	err := db.store.FindAll(ctx, filter, nil, nil, &deliveries)
	if err != nil {
		return nil, err
	}

	return deliveries, nil
}

func (db *eventDeliveryRepo) FindEventDeliveriesByEventID(ctx context.Context,
	eventID string,
) ([]datastore.EventDelivery, error) {
	ctx = db.setCollectionInContext(ctx)

	filter := bson.M{"event_id": eventID}
	var deliveries []datastore.EventDelivery

	err := db.store.FindAll(ctx, filter, nil, nil, deliveries)
	if err != nil {
		return nil, err
	}

	return deliveries, nil
}

func (db *eventDeliveryRepo) CountDeliveriesByStatus(ctx context.Context,
	status datastore.EventDeliveryStatus, searchParams datastore.SearchParams,
) (int64, error) {
	ctx = db.setCollectionInContext(ctx)

	filter := bson.M{
		"status":     status,
		"created_at": getCreatedDateFilter(searchParams),
	}

	count, err := db.store.Count(ctx, filter)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (db *eventDeliveryRepo) UpdateStatusOfEventDelivery(ctx context.Context,
	e datastore.EventDelivery, status datastore.EventDeliveryStatus,
) error {
	ctx = db.setCollectionInContext(ctx)

	filter := bson.M{"uid": e.UID}
	update := bson.M{
		"$set": bson.M{
			"status":     status,
			"updated_at": primitive.NewDateTimeFromTime(time.Now()),
		},
	}

	return db.store.UpdateOne(ctx, filter, update)
}

func (db *eventDeliveryRepo) UpdateStatusOfEventDeliveries(ctx context.Context, ids []string, status datastore.EventDeliveryStatus) error {
	ctx = db.setCollectionInContext(ctx)

	filter := bson.M{
		"uid": bson.M{
			"$in": ids,
		},
		"deleted_at": 0,
	}

	update := bson.M{
		"$set": bson.M{
			"status":     status,
			"updated_at": primitive.NewDateTimeFromTime(time.Now()),
		},
	}

	return db.store.UpdateMany(ctx, filter, update, false)
}

func (db *eventDeliveryRepo) UpdateEventDeliveryWithAttempt(ctx context.Context,
	e datastore.EventDelivery, attempt datastore.DeliveryAttempt,
) error {
	ctx = db.setCollectionInContext(ctx)

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

	return db.store.UpdateOne(ctx, filter, update)
}

func (db *eventDeliveryRepo) LoadEventDeliveriesPaged(ctx context.Context, groupID, appID, eventID string, status []datastore.EventDeliveryStatus, searchParams datastore.SearchParams, pageable datastore.Pageable) ([]datastore.EventDelivery, datastore.PaginationData, error) {
	filter := getFilter(groupID, appID, eventID, status, searchParams)
	ctx = db.setCollectionInContext(ctx)

	var eventDeliveries []datastore.EventDelivery
	pagination, err := db.store.FindMany(ctx, filter, nil, nil,
		int64(pageable.Page), int64(pageable.PerPage), &eventDeliveries)
	if err != nil {
		return eventDeliveries, datastore.PaginationData{}, err
	}

	if eventDeliveries == nil {
		eventDeliveries = make([]datastore.EventDelivery, 0)
	}

	return eventDeliveries, pagination, nil
}

func (db *eventDeliveryRepo) CountEventDeliveries(ctx context.Context, groupID, appID, eventID string, status []datastore.EventDeliveryStatus, searchParams datastore.SearchParams) (int64, error) {
	filter := getFilter(groupID, appID, eventID, status, searchParams)
	ctx = db.setCollectionInContext(ctx)

	var count int64
	count, err := db.store.Count(ctx, filter)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (db *eventDeliveryRepo) DeleteGroupEventDeliveries(ctx context.Context, filter *datastore.EventDeliveryFilter, hardDelete bool) error {
	ctx = db.setCollectionInContext(ctx)

	update := bson.M{
		"deleted_at": primitive.NewDateTimeFromTime(time.Now()),
	}

	f := bson.M{
		"group_id":   filter.GroupID,
		"deleted_at": 0,
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
	ctx = db.setCollectionInContext(ctx)

	filter := bson.M{
		"app_id":     appId,
		"device_id":  deviceId,
		"status":     datastore.DiscardedEventStatus,
		"created_at": getCreatedDateFilter(searchParams),
		"deleted_at": 0,
	}

	deliveries := make([]datastore.EventDelivery, 0)

	err := db.store.FindAll(ctx, filter, nil, nil, &deliveries)
	if err != nil {
		return deliveries, err
	}

	return deliveries, nil
}

func (db *eventDeliveryRepo) setCollectionInContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, datastore.CollectionCtx, datastore.EventDeliveryCollection)
}

func getFilter(groupID string, appID string, eventID string, status []datastore.EventDeliveryStatus, searchParams datastore.SearchParams) bson.M {
	filter := bson.M{
		"deleted_at": 0,
		"created_at": getCreatedDateFilter(searchParams),
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
