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

type eventRepo struct {
	inner *mongo.Collection
	store datastore.Store
}

func NewEventRepository(db *mongo.Database, store datastore.Store) datastore.EventRepository {
	return &eventRepo{
		inner: db.Collection(EventCollection),
		store: store,
	}
}

var dailyIntervalFormat = "%Y-%m-%d" // 1 day
var weeklyIntervalFormat = "%Y-%m"   // 1 week
var monthlyIntervalFormat = "%Y-%m"  // 1 month
var yearlyIntervalFormat = "%Y"      // 1 month

func (db *eventRepo) CreateEvent(ctx context.Context, message *datastore.Event) error {

	message.ID = primitive.NewObjectID()

	if util.IsStringEmpty(message.ProviderID) {
		message.ProviderID = message.AppID
	}
	if util.IsStringEmpty(message.UID) {
		message.UID = uuid.New().String()
	}

	err := db.store.Save(ctx, message, nil)
	return err
}

func (db *eventRepo) CountGroupMessages(ctx context.Context, groupID string) (int64, error) {
	filter := bson.M{
		"group_id":        groupID,
		"document_status": datastore.ActiveDocumentStatus,
	}

	count, err := db.store.Count(ctx, filter)
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
			"document_status": datastore.ActiveDocumentStatus,
		},
	}

	filter := bson.M{"group_id": groupID}
	err := db.store.UpdateMany(ctx, filter, update)
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
		{Key: "group_id", Value: groupID},
		{Key: "document_status", Value: datastore.ActiveDocumentStatus},
		{Key: "created_at", Value: bson.D{
			{Key: "$gte", Value: primitive.NewDateTimeFromTime(time.Unix(start, 0))},
			{Key: "$lte", Value: primitive.NewDateTimeFromTime(time.Unix(end, 0))},
		}},
	}}}

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
	var eventsIntervals []datastore.EventInterval

	err := db.store.Aggregate(ctx, mongo.Pipeline{matchStage, groupStage, sortStage}, &eventsIntervals, false)
	if err != nil {
		log.WithError(err).Errorln("aggregate error")
		return nil, err
	}
	if eventsIntervals == nil {
		eventsIntervals = make([]datastore.EventInterval, 0)
	}

	return eventsIntervals, nil
}

func (db *eventRepo) FindEventByID(ctx context.Context, id string) (*datastore.Event, error) {
	m := new(datastore.Event)

	err := db.store.FindByID(ctx, id, nil, &m)

	if errors.Is(err, mongo.ErrNoDocuments) {
		err = datastore.ErrEventNotFound
	}

	return m, err
}

func (db *eventRepo) FindEventsByIDs(ctx context.Context, ids []string) ([]datastore.Event, error) {
	m := make([]datastore.Event, 0)

	filter := bson.M{"uid": bson.M{"$in": ids}, "document_status": datastore.ActiveDocumentStatus}

	err := db.store.FindMany(ctx, filter, nil, nil, 0, 0, &m)
	if err != nil {
		return nil, err
	}
	return m, err
}

func (db *eventRepo) LoadEventsPaged(ctx context.Context, groupID string, appId string, searchParams datastore.SearchParams, pageable datastore.Pageable) ([]datastore.Event, datastore.PaginationData, error) {
	filter := bson.M{"document_status": datastore.ActiveDocumentStatus, "created_at": getCreatedDateFilter(searchParams)}

	hasAppFilter := !util.IsStringEmpty(appId)
	hasGroupFilter := !util.IsStringEmpty(groupID)

	if hasAppFilter && hasGroupFilter {
		filter = bson.M{"group_id": groupID, "app_id": appId, "document_status": datastore.ActiveDocumentStatus,
			"created_at": getCreatedDateFilter(searchParams)}
	} else if hasAppFilter {
		filter = bson.M{"app_id": appId, "document_status": datastore.ActiveDocumentStatus,
			"created_at": getCreatedDateFilter(searchParams)}
	} else if hasGroupFilter {
		filter = bson.M{"group_id": groupID, "document_status": datastore.ActiveDocumentStatus,
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
