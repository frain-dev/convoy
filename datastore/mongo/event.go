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
	"go.mongodb.org/mongo-driver/mongo/options"
)

type eventRepo struct {
	inner *mongo.Collection
}

func NewEventRepository(db *mongo.Database) datastore.EventRepository {
	return &eventRepo{
		inner: db.Collection(EventCollection),
	}
}

var dailyIntervalFormat = "%Y-%m-%d" // 1 day
var weeklyIntervalFormat = "%Y-%m"   // 1 week
var monthlyIntervalFormat = "%Y-%m"  // 1 month
var yearlyIntervalFormat = "%Y"      // 1 month

func (db *eventRepo) CreateEvent(ctx context.Context,
	message *datastore.Event) error {

	message.ID = primitive.NewObjectID()

	if util.IsStringEmpty(message.ProviderID) {
		message.ProviderID = message.AppMetadata.UID
	}
	if util.IsStringEmpty(message.UID) {
		message.UID = uuid.New().String()
	}

	_, err := db.inner.InsertOne(ctx, message)
	return err
}

func (db *eventRepo) CountGroupMessages(ctx context.Context, groupID string) (int64, error) {
	filter := bson.M{
		"app_metadata.group_id": groupID,
		"document_status": bson.M{
			"$ne": datastore.DeletedDocumentStatus,
		},
	}

	count, err := db.inner.CountDocuments(ctx, filter)
	if err != nil {
		log.WithError(err).Errorf("failed to count events in group %s", groupID)
		return 0, err
	}
	return count, nil
}

func (db *eventRepo) DeleteGroupEvents(ctx context.Context, groupID string) error {
	update := bson.M{
		"$set": bson.M{
			"deleted_at":      primitive.NewDateTimeFromTime(time.Now()),
			"document_status": datastore.DeletedDocumentStatus,
		},
	}

	filter := bson.M{"app_metadata.group_id": groupID}
	_, err := db.inner.UpdateMany(ctx, filter, update)
	if err != nil {
		return err
	}

	return nil
}

func (db *eventRepo) LoadEventIntervals(ctx context.Context, groupID string, searchParams datastore.SearchParams, period datastore.Period, interval int) ([]datastore.EventInterval, error) {

	start := searchParams.CreatedAtStart
	end := searchParams.CreatedAtEnd
	if end == 0 || end < searchParams.CreatedAtStart {
		end = start
	}

	matchStage := bson.D{{Key: "$match", Value: bson.D{
		{Key: "app_metadata.group_id", Value: groupID},
		{Key: "document_status", Value: bson.D{
			{Key: "$ne", Value: datastore.DeletedDocumentStatus},
		}},
		{Key: "created_at", Value: bson.D{
			{Key: "$gte", Value: primitive.NewDateTimeFromTime(time.Unix(start, 0))},
			{Key: "$lte", Value: primitive.NewDateTimeFromTime(time.Unix(end, 0))},
		},
		}},
	}}

	var timeComponent string
	var format string
	switch period {
	case datastore.Daily:
		timeComponent = "$dayOfYear"
		format = dailyIntervalFormat
	case datastore.Weekly:
		timeComponent = "$week"
		format = weeklyIntervalFormat
	case datastore.Monthly:
		timeComponent = "$month"
		format = monthlyIntervalFormat
	case datastore.Yearly:
		timeComponent = "$year"
		format = yearlyIntervalFormat
	default:
		return nil, errors.New("specified data cannot be generated for period")
	}
	groupStage := bson.D{
		{Key: "$group", Value: bson.D{
			{Key: "_id",
				Value: bson.D{
					{Key: "total_time",
						Value: bson.D{{Key: "$dateToString", Value: bson.D{{Key: "date", Value: "$created_at"}, {Key: "format", Value: format}}}},
					},
					{Key: "index", Value: bson.D{{Key: "$trunc", Value: bson.D{{Key: "$divide", Value: bson.A{
						bson.D{{Key: timeComponent, Value: "$created_at"}},
						interval,
					},
					}}}}},
				},
			},
			{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
		},
		},
	}
	sortStage := bson.D{{Key: "$sort", Value: bson.D{primitive.E{Key: "_id", Value: 1}}}}

	data, err := db.inner.Aggregate(ctx, mongo.Pipeline{matchStage, groupStage, sortStage})
	if err != nil {
		log.WithError(err).Errorln("aggregate error")
		return nil, err
	}
	var eventsIntervals []datastore.EventInterval
	if err = data.All(ctx, &eventsIntervals); err != nil {
		log.WithError(err).Error("marshal error")
		return nil, err
	}
	if eventsIntervals == nil {
		eventsIntervals = make([]datastore.EventInterval, 0)
	}

	return eventsIntervals, nil
}

func (db *eventRepo) LoadEventsPagedByAppId(ctx context.Context, appId string, searchParams datastore.SearchParams, pageable datastore.Pageable) ([]datastore.Event, datastore.PaginationData, error) {
	filter := bson.M{"app_id": appId, "document_status": bson.M{"$ne": datastore.DeletedDocumentStatus}, "created_at": getCreatedDateFilter(searchParams)}

	var messages []datastore.Event
	paginatedData, err := pager.New(db.inner).Context(ctx).Limit(int64(pageable.PerPage)).Page(int64(pageable.Page)).Sort("created_at", pageable.Sort).Filter(filter).Decode(&messages).Find()
	if err != nil {
		return messages, datastore.PaginationData{}, err
	}

	if messages == nil {
		messages = make([]datastore.Event, 0)
	}

	return messages, datastore.PaginationData(paginatedData.Pagination), nil
}

func (db *eventRepo) FindEventByID(ctx context.Context, id string) (*datastore.Event, error) {
	m := new(datastore.Event)

	filter := bson.M{"uid": id, "document_status": bson.M{"$ne": datastore.DeletedDocumentStatus}}

	err := db.inner.FindOne(ctx, filter).
		Decode(&m)
	if errors.Is(err, mongo.ErrNoDocuments) {
		err = datastore.ErrEventNotFound
	}

	return m, err
}

func (db *eventRepo) LoadEventsScheduledForPosting(ctx context.Context) ([]datastore.Event, error) {

	filter := bson.M{"status": datastore.ScheduledEventStatus,
		"document_status":         bson.M{"$ne": datastore.DeletedDocumentStatus},
		"metadata.next_send_time": bson.M{"$lte": primitive.NewDateTimeFromTime(time.Now())}}

	return db.loadEventsByFilter(ctx, filter, nil)
}

func (db *eventRepo) loadEventsByFilter(ctx context.Context, filter bson.M, findOptions *options.FindOptions) ([]datastore.Event, error) {
	messages := make([]datastore.Event, 0)
	cur, err := db.inner.Find(ctx, filter, findOptions)
	if err != nil {
		return messages, err
	}

	for cur.Next(ctx) {
		var message datastore.Event
		if err := cur.Decode(&message); err != nil {
			return messages, err
		}

		messages = append(messages, message)
	}

	if err := cur.Err(); err != nil {
		return nil, err
	}

	if err := cur.Close(ctx); err != nil {
		return messages, err
	}

	return messages, nil
}

func (db *eventRepo) LoadEventsForPostingRetry(ctx context.Context) ([]datastore.Event, error) {

	filter := bson.M{
		"$and": []bson.M{
			{"status": datastore.RetryEventStatus},
			{"metadata.next_send_time": bson.M{"$lte": primitive.NewDateTimeFromTime(time.Now())}},
			{"document_status": bson.M{"$ne": datastore.DeletedDocumentStatus}},
		},
	}

	return db.loadEventsByFilter(ctx, filter, nil)
}

func (db *eventRepo) LoadAbandonedEventsForPostingRetry(ctx context.Context) ([]datastore.Event, error) {
	filter := bson.M{
		"$and": []bson.M{
			{"status": datastore.ProcessingEventStatus},
			{"metadata.next_send_time": bson.M{"$lte": primitive.NewDateTimeFromTime(time.Now())}},
			{"document_status": bson.M{"$ne": datastore.DeletedDocumentStatus}},
		},
	}

	return db.loadEventsByFilter(ctx, filter, nil)
}

func (db *eventRepo) LoadEventsPaged(ctx context.Context, groupID string, appId string, searchParams datastore.SearchParams, pageable datastore.Pageable) ([]datastore.Event, datastore.PaginationData, error) {
	filter := bson.M{"document_status": bson.M{"$ne": datastore.DeletedDocumentStatus}, "created_at": getCreatedDateFilter(searchParams)}

	hasAppFilter := !util.IsStringEmpty(appId)
	hasGroupFilter := !util.IsStringEmpty(groupID)

	if hasAppFilter && hasGroupFilter {
		filter = bson.M{"app_metadata.group_id": groupID, "app_metadata.uid": appId, "document_status": bson.M{"$ne": datastore.DeletedDocumentStatus},
			"created_at": getCreatedDateFilter(searchParams)}
	} else if hasAppFilter {
		filter = bson.M{"app_id": appId, "document_status": bson.M{"$ne": datastore.DeletedDocumentStatus},
			"created_at": getCreatedDateFilter(searchParams)}
	} else if hasGroupFilter {
		filter = bson.M{"app_metadata.group_id": groupID, "document_status": bson.M{"$ne": datastore.DeletedDocumentStatus},
			"created_at": getCreatedDateFilter(searchParams)}
	}

	var messages []datastore.Event
	paginatedData, err := pager.New(db.inner).Context(ctx).Limit(int64(pageable.PerPage)).Page(int64(pageable.Page)).Sort("created_at", pageable.Sort).Filter(filter).Decode(&messages).Find()
	if err != nil {
		return messages, datastore.PaginationData{}, err
	}

	if messages == nil {
		messages = make([]datastore.Event, 0)
	}

	return messages, datastore.PaginationData(paginatedData.Pagination), nil
}

func getCreatedDateFilter(searchParams datastore.SearchParams) bson.M {
	return bson.M{"$gte": primitive.NewDateTimeFromTime(time.Unix(searchParams.CreatedAtStart, 0)), "$lte": primitive.NewDateTimeFromTime(time.Unix(searchParams.CreatedAtEnd, 0))}
}
