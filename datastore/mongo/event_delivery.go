package mongo

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
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

func (db *eventDeliveryRepo) CreateEventDelivery(ctx context.Context, eventDelivery *datastore.EventDelivery) error {
	ctx = db.setCollectionInContext(ctx)

	if util.IsStringEmpty(eventDelivery.UID) {
		eventDelivery.UID = uuid.New().String()
	}

	return db.store.Save(ctx, eventDelivery, nil)
}

func (db *eventDeliveryRepo) FindEventDeliveryByID(ctx context.Context, uid string) (*datastore.EventDelivery, error) {
	var eventDelivery *datastore.EventDelivery

	ctx = db.setCollectionInContext(ctx)

	matchStage := bson.D{
		{
			Key: "$match",
			Value: bson.D{
				{Key: "uid", Value: uid},
				{Key: "deleted_at", Value: nil},
			},
		},
	}

	endpointLookupStage := bson.D{
		{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: datastore.EndpointCollection},
			{Key: "localField", Value: "endpoint_id"},
			{Key: "foreignField", Value: "uid"},
			{Key: "as", Value: "endpoint"},
			{Key: "pipeline", Value: bson.A{
				bson.D{
					{
						Key: "$project",
						Value: bson.D{
							{Key: "uid", Value: 1},
							{Key: "title", Value: 1},
							{Key: "project_id", Value: 1},
							{Key: "support_email", Value: 1},
							{Key: "target_url", Value: 1},
						},
					},
				},
			}},
		}},
	}

	eventLookupStage := bson.D{
		{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: datastore.EventCollection},
			{Key: "localField", Value: "event_id"},
			{Key: "foreignField", Value: "uid"},
			{Key: "as", Value: "event"},
			{Key: "pipeline", Value: bson.A{
				bson.D{
					{
						Key: "$project",
						Value: bson.D{
							{Key: "uid", Value: 1},
							{Key: "event_type", Value: 1},
						},
					},
				},
			}},
		}},
	}

	deviceLookupStage := bson.D{
		{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: datastore.DeviceCollection},
			{Key: "localField", Value: "device_id"},
			{Key: "foreignField", Value: "uid"},
			{Key: "as", Value: "device"},
			{
				Key: "pipeline",
				Value: bson.A{
					bson.D{
						{
							Key: "$project",
							Value: bson.D{
								{Key: "uid", Value: 1},
								{Key: "host_name", Value: 1},
							},
						},
					},
				},
			},
		}},
	}

	projectStage := bson.D{
		{Key: "$addFields", Value: bson.M{
			"device_metadata": bson.M{
				"$first": "$device",
			},
			"event_metadata": bson.M{
				"$first": "$event",
			},
			"endpoint_metadata": bson.M{
				"$first": "$endpoint",
			},
		}},
	}

	setStage := bson.D{
		{
			Key: "$set",
			Value: bson.D{
				{Key: "cli_metadata.host_name", Value: "$device_metadata.host_name"},
			},
		},
	}

	unsetStage := bson.D{
		{
			Key: "$unset",
			Value: []string{
				"device",
				"endpoint",
				"event",
				"endpoint_metadata.secrets",
				"endpoint_metadata.authentication",
			},
		},
	}

	pipeline := mongo.Pipeline{
		matchStage,
		endpointLookupStage,
		eventLookupStage,
		deviceLookupStage,
		projectStage,
		setStage,
		unsetStage,
	}

	var eventDeliveries []datastore.EventDelivery
	err := db.store.Aggregate(ctx, pipeline, &eventDeliveries, false)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			err = datastore.ErrEventDeliveryNotFound
		}
		return nil, err
	}

	if len(eventDeliveries) == 0 {
		return nil, datastore.ErrEventDeliveryNotFound
	}
	eventDelivery = &eventDeliveries[0]

	return eventDelivery, nil
}

func (db *eventDeliveryRepo) FindEventDeliveriesByIDs(ctx context.Context, ids []string) ([]datastore.EventDelivery, error) {
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

func (db *eventDeliveryRepo) FindEventDeliveriesByEventID(ctx context.Context, eventID string) ([]datastore.EventDelivery, error) {
	ctx = db.setCollectionInContext(ctx)

	filter := bson.M{"event_id": eventID}
	var deliveries []datastore.EventDelivery

	err := db.store.FindAll(ctx, filter, nil, nil, deliveries)
	if err != nil {
		return nil, err
	}

	return deliveries, nil
}

func (db *eventDeliveryRepo) CountDeliveriesByStatus(ctx context.Context, status datastore.EventDeliveryStatus, searchParams datastore.SearchParams) (int64, error) {
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

func (db *eventDeliveryRepo) UpdateStatusOfEventDelivery(ctx context.Context, e datastore.EventDelivery, status datastore.EventDeliveryStatus) error {
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
	}

	update := bson.M{
		"$set": bson.M{
			"status":     status,
			"updated_at": primitive.NewDateTimeFromTime(time.Now()),
		},
	}

	return db.store.UpdateMany(ctx, filter, update, false)
}

func (db *eventDeliveryRepo) UpdateEventDeliveryWithAttempt(ctx context.Context, e datastore.EventDelivery, attempt datastore.DeliveryAttempt) error {
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

func (db *eventDeliveryRepo) LoadEventDeliveriesPaged(ctx context.Context, projectID string, endpointIDs []string, eventID string, status []datastore.EventDeliveryStatus, searchParams datastore.SearchParams, pageable datastore.Pageable) ([]datastore.EventDelivery, datastore.PaginationData, error) {
	filter := getFilter(projectID, endpointIDs, eventID, status, searchParams)
	ctx = db.setCollectionInContext(ctx)

	matchStage := bson.D{{Key: "$match", Value: mToD(filter)}}
	endpointLookupStage := bson.D{
		{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: datastore.EndpointCollection},
			{Key: "localField", Value: "endpoint_id"},
			{Key: "foreignField", Value: "uid"},
			{Key: "as", Value: "endpoint_metadata"},
			{Key: "pipeline", Value: bson.A{
				bson.D{
					{
						Key: "$project",
						Value: bson.D{
							{Key: "uid", Value: 1},
							{Key: "title", Value: 1},
							{Key: "project_id", Value: 1},
							{Key: "support_email", Value: 1},
							{Key: "target_url", Value: 1},
						},
					},
				},
			}},
		}},
	}
	unwindAppStage := bson.D{{Key: "$unwind", Value: bson.D{{Key: "path", Value: "$endpoint_metadata"}, {Key: "preserveNullAndEmptyArrays", Value: true}}}}

	eventLookupStage := bson.D{
		{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: datastore.EventCollection},
			{Key: "localField", Value: "event_id"},
			{Key: "foreignField", Value: "uid"},
			{Key: "as", Value: "event_metadata"},
			{Key: "pipeline", Value: bson.A{
				bson.D{
					{
						Key: "$project",
						Value: bson.D{
							{Key: "uid", Value: 1},
							{Key: "event_type", Value: 1},
						},
					},
				},
			}},
		}},
	}
	unwindEventStage := bson.D{{Key: "$unwind", Value: bson.D{{Key: "path", Value: "$event_metadata"}, {Key: "preserveNullAndEmptyArrays", Value: true}}}}

	deviceLookupStage := bson.D{
		{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: datastore.DeviceCollection},
			{Key: "localField", Value: "device_id"},
			{Key: "foreignField", Value: "uid"},
			{Key: "as", Value: "device_metadata"},
			{
				Key: "pipeline",
				Value: bson.A{
					bson.D{
						{
							Key: "$project",
							Value: bson.D{
								{Key: "uid", Value: 1},
								{Key: "host_name", Value: 1},
							},
						},
					},
				},
			},
		}},
	}
	unwindDeviceStage := bson.D{{Key: "$unwind", Value: bson.D{{Key: "path", Value: "$device_metadata"}, {Key: "preserveNullAndEmptyArrays", Value: true}}}}

	setStage := bson.D{
		{
			Key: "$set",
			Value: bson.D{
				{Key: "cli_metadata.host_name", Value: "$device_metadata.host_name"},
			},
		},
	}

	unsetStage := bson.D{
		{
			Key: "$unset",
			Value: []string{
				"app_metadata.endpoints",
				"endpoint_metadata.secrets",
				"endpoint_metadata.authentication",
			},
		},
	}

	skipStage := bson.D{{Key: "$skip", Value: getSkip(pageable.Page, pageable.PerPage)}}
	sortStage := bson.D{{Key: "$sort", Value: bson.D{{Key: "created_at", Value: -1}}}}
	limitStage := bson.D{{Key: "$limit", Value: pageable.PerPage}}

	pipeline := mongo.Pipeline{
		matchStage,
		sortStage,
		skipStage,
		limitStage,
		endpointLookupStage,
		unwindAppStage,
		eventLookupStage,
		unwindEventStage,
		deviceLookupStage,
		unwindDeviceStage,
		setStage,
		unsetStage,
	}

	var eventDeliveries []datastore.EventDelivery
	err := db.store.Aggregate(ctx, pipeline, &eventDeliveries, false)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			err = datastore.ErrEventDeliveryNotFound
		}
		return nil, datastore.PaginationData{}, err
	}

	var count int64
	if eventDeliveries == nil {
		eventDeliveries = make([]datastore.EventDelivery, 0)
	} else {
		count, err = db.store.Count(ctx, filter)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}
	}

	pagination := datastore.PaginationData{
		Total:     count,
		Page:      int64(pageable.Page),
		PerPage:   int64(pageable.PerPage),
		Prev:      int64(getPrevPage(pageable.Page)),
		Next:      int64(pageable.Page + 1),
		TotalPage: int64(math.Ceil(float64(count) / float64(pageable.PerPage))),
	}

	return eventDeliveries, pagination, nil
}

func (db *eventDeliveryRepo) CountEventDeliveries(ctx context.Context, projectID string, endpointIDs []string, eventID string, status []datastore.EventDeliveryStatus, searchParams datastore.SearchParams) (int64, error) {
	filter := getFilter(projectID, endpointIDs, eventID, status, searchParams)
	ctx = db.setCollectionInContext(ctx)

	var count int64
	count, err := db.store.Count(ctx, filter)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (db *eventDeliveryRepo) DeleteProjectEventDeliveries(ctx context.Context, filter *datastore.EventDeliveryFilter, hardDelete bool) error {
	ctx = db.setCollectionInContext(ctx)

	update := bson.M{
		"deleted_at": primitive.NewDateTimeFromTime(time.Now()),
	}

	f := bson.M{
		"project_id": filter.ProjectID,
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

func (db *eventDeliveryRepo) FindDiscardedEventDeliveries(ctx context.Context, endpointID, deviceId string, searchParams datastore.SearchParams) ([]datastore.EventDelivery, error) {
	ctx = db.setCollectionInContext(ctx)

	filter := bson.M{
		"endpoint_id": endpointID,
		"device_id":   deviceId,
		"status":      datastore.DiscardedEventStatus,
		"created_at":  getCreatedDateFilter(searchParams),
	}

	deliveries := make([]datastore.EventDelivery, 0)

	err := db.store.FindAll(ctx, filter, nil, nil, &deliveries)
	if err != nil {
		return deliveries, err
	}

	return deliveries, nil
}

func (db *eventDeliveryRepo) LoadEventDeliveriesIntervals(ctx context.Context, projectID string, searchParams datastore.SearchParams, period datastore.Period, interval int) ([]datastore.EventInterval, error) {
	ctx = db.setCollectionInContext(ctx)

	start := searchParams.CreatedAtStart
	end := searchParams.CreatedAtEnd
	if end == 0 || end < searchParams.CreatedAtStart {
		end = start
	}

	matchStage := bson.D{{Key: "$match", Value: bson.D{
		{Key: "project_id", Value: projectID},
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

func (db *eventDeliveryRepo) setCollectionInContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, datastore.CollectionCtx, datastore.EventDeliveryCollection)
}

func getFilter(projectID string, endpointIDs []string, eventID string, status []datastore.EventDeliveryStatus, searchParams datastore.SearchParams) bson.M {
	filter := bson.M{
		"created_at": getCreatedDateFilter(searchParams),
	}

	hasEndpointFilter := len(endpointIDs) > 0
	hasProjectFilter := !util.IsStringEmpty(projectID)
	hasEventFilter := !util.IsStringEmpty(eventID)
	hasStatusFilter := len(status) > 0

	if hasEndpointFilter {
		filter["endpoint_id"] = bson.M{"$in": endpointIDs}
	}

	if hasProjectFilter {
		filter["project_id"] = projectID
	}

	if hasEventFilter {
		filter["event_id"] = eventID
	}

	if hasStatusFilter {
		filter["status"] = bson.M{"$in": status}
	}

	return filter
}

// mToD created a bson.D from the entries in M
func mToD(m bson.M) bson.D {
	d := bson.D{}

	for k, v := range m {
		switch n := v.(type) {
		case bson.M:
			d = append(d, bson.E{Key: k, Value: mToD(n)})
		default:
			d = append(d, bson.E{Key: k, Value: n})
		}
	}

	return d
}

// dToM creates a map from the elements of the D.
func DToM(d bson.D) bson.M {
	m := make(bson.M, len(d))
	for _, e := range d {
		if v, ok := e.Value.(bson.D); ok {
			m[e.Key] = v.Map()
			continue
		}
		m[e.Key] = e.Value
	}
	return m
}
