package mongo

import (
	"context"
	"errors"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type eventRepo struct {
	store datastore.Store
}

func NewEventRepository(store datastore.Store) datastore.EventRepository {
	return &eventRepo{
		store: store,
	}
}

var (
	dailyIntervalFormat   = "%Y-%m-%d" // 1 day
	weeklyIntervalFormat  = "%Y-%m"    // 1 week
	monthlyIntervalFormat = "%Y-%m"    // 1 month
	yearlyIntervalFormat  = "%Y"       // 1 month
)

func (db *eventRepo) CreateEvent(ctx context.Context, message *datastore.Event) error {
	ctx = db.setCollectionInContext(ctx)

	message.ID = primitive.NewObjectID()

	if util.IsStringEmpty(message.ProviderID) {
		message.ProviderID = message.AppID
	}
	if util.IsStringEmpty(message.UID) {
		message.UID = uuid.New().String()
	}

	return db.store.Save(ctx, message, nil)
}

func (db *eventRepo) CountGroupMessages(ctx context.Context, groupID string) (int64, error) {
	ctx = db.setCollectionInContext(ctx)
	return db.store.Count(ctx, bson.M{"group_id": groupID})
}

func (db *eventRepo) DeleteGroupEvents(ctx context.Context, filter *datastore.EventFilter, hardDelete bool) error {
	ctx = db.setCollectionInContext(ctx)

	update := bson.M{
		"deleted_at": primitive.NewDateTimeFromTime(time.Now()),
	}

	f := bson.M{
		"group_id":   filter.GroupID,
		"deleted_at": nil,
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

func (db *eventRepo) LoadEventIntervals(ctx context.Context, groupID string, searchParams datastore.SearchParams, period datastore.Period, interval int) ([]datastore.EventInterval, error) {
	ctx = db.setCollectionInContext(ctx)

	start := searchParams.CreatedAtStart
	end := searchParams.CreatedAtEnd
	if end == 0 || end < searchParams.CreatedAtStart {
		end = start
	}

	matchStage := bson.D{{Key: "$match", Value: bson.D{
		{Key: "group_id", Value: groupID},
		{Key: "deleted_at", Value: nil},
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
		{
			Key: "$group", Value: bson.D{
				{
					Key: "_id",
					Value: bson.D{
						{
							Key:   "total_time",
							Value: bson.D{{Key: "$dateToString", Value: bson.D{{Key: "date", Value: "$created_at"}, {Key: "format", Value: format}}}},
						},
						{Key: "index", Value: bson.D{{Key: "$trunc", Value: bson.D{{
							Key: "$divide", Value: bson.A{
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
	ctx = db.setCollectionInContext(ctx)

	m := new(datastore.Event)

	err := db.store.FindByID(ctx, id, nil, m)

	if errors.Is(err, mongo.ErrNoDocuments) {
		err = datastore.ErrEventNotFound
	}

	return m, err
}

func (db *eventRepo) FindEventsByIDs(ctx context.Context, ids []string) ([]datastore.Event, error) {
	ctx = db.setCollectionInContext(ctx)

	var event []datastore.Event

	filter := bson.M{"uid": bson.M{"$in": ids}}

	_, err := db.store.FindMany(ctx, filter, nil, nil, 0, 0, &event)
	if err != nil {
		return nil, err
	}

	return event, err
}

func (db *eventRepo) LoadEventsPaged(ctx context.Context, f *datastore.Filter) ([]datastore.Event, datastore.PaginationData, error) {
	ctx = db.setCollectionInContext(ctx)

	filter := bson.M{"deleted_at": nil, "created_at": getCreatedDateFilter(f.SearchParams)}

	if !util.IsStringEmpty(f.AppID) {
		filter["app_id"] = f.AppID
	}

	if !util.IsStringEmpty(f.Group.UID) {
		filter["group_id"] = f.Group.UID
	}

	if !util.IsStringEmpty(f.SourceID) {
		filter["source_id"] = f.SourceID
	}

	var events []datastore.Event
	pagination, err := db.store.FindMany(ctx, filter, nil, nil,
		int64(f.Pageable.Page), int64(f.Pageable.PerPage), &events)
	if err != nil {
		return events, datastore.PaginationData{}, err
	}

	if events == nil {
		events = make([]datastore.Event, 0)
	}

	return events, pagination, nil
}

func getCreatedDateFilter(searchParams datastore.SearchParams) bson.M {
	return bson.M{"$gte": primitive.NewDateTimeFromTime(time.Unix(searchParams.CreatedAtStart, 0)), "$lte": primitive.NewDateTimeFromTime(time.Unix(searchParams.CreatedAtEnd, 0))}
}

func (db *eventRepo) setCollectionInContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, datastore.CollectionCtx, datastore.EventCollection)
}
