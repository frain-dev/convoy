package badger

import (
	"context"
	"math"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	"github.com/timshannon/badgerhold/v4"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type eventDeliveryRepo struct {
	db *badgerhold.Store
}

func NewEventDeliveryRepository(db *badgerhold.Store) datastore.EventDeliveryRepository {
	return &eventDeliveryRepo{db: db}
}

func (e *eventDeliveryRepo) CreateEventDelivery(ctx context.Context, delivery *datastore.EventDelivery) error {
	return e.db.Upsert(delivery.UID, delivery)
}

func (e *eventDeliveryRepo) FindEventDeliveryByID(ctx context.Context, uid string) (*datastore.EventDelivery, error) {
	var delivery datastore.EventDelivery
	err := e.db.Get(uid, &delivery)

	return &delivery, err
}

func (e *eventDeliveryRepo) FindEventDeliveriesByIDs(ctx context.Context, uids []string) ([]datastore.EventDelivery, error) {
	deliveries := make([]datastore.EventDelivery, 0, len(uids))

	s := make([]interface{}, len(uids))
	for i, uid := range uids {
		s[i] = uid
	}

	err := e.db.Find(&deliveries, badgerhold.Where("UID").In(s...))

	return deliveries, err
}

func (e *eventDeliveryRepo) FindEventDeliveriesByEventID(ctx context.Context, eventID string) ([]datastore.EventDelivery, error) {
	var deliveries []datastore.EventDelivery

	err := e.db.Find(&deliveries, badgerhold.Where("EventMetadata.UID").Eq(eventID))

	return deliveries, err
}

func (e *eventDeliveryRepo) UpdateStatusOfEventDelivery(ctx context.Context, delivery datastore.EventDelivery, status datastore.EventDeliveryStatus) error {
	delivery.Status = status
	delivery.UpdatedAt = primitive.NewDateTimeFromTime(time.Now())

	return e.db.Update(delivery.UID, delivery)
}

func (e *eventDeliveryRepo) UpdateEventDeliveryWithAttempt(ctx context.Context, delivery datastore.EventDelivery, attempt datastore.DeliveryAttempt) error {
	delivery.DeliveryAttempts = append(delivery.DeliveryAttempts, attempt)

	return e.db.Update(delivery.UID, delivery)
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

	var deliveries []datastore.EventDelivery = make([]datastore.EventDelivery, 0)
	var pg datastore.PaginationData

	q := e.generateQuery(f).Skip(lowerBound).Limit(pageable.PerPage).SortBy("CreatedAt")
	if pageable.Sort == -1 {
		q.Reverse()
	}

	err := e.db.Find(&deliveries, q)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	total, err := e.db.Count(&datastore.EventDelivery{}, e.generateQuery(f))
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	pg = datastore.PaginationData{
		Total:     int64(total),
		Page:      int64(pageable.Page),
		PerPage:   int64(pageable.PerPage),
		Prev:      int64(prevPage),
		Next:      int64(pageable.Page + 1),
		TotalPage: int64(math.Ceil(float64(total) / float64(pageable.PerPage))),
	}

	return deliveries, pg, err
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

func (e *eventDeliveryRepo) generateQuery(f *filter) *badgerhold.Query {
	qFunc := badgerhold.Where

	if f.hasAppFilter {
		qFunc = qFunc("AppMetadata.UID").Eq(f.appID).And
	}

	if f.hasGroupFilter {
		qFunc = qFunc("AppMetadata.GroupID").Eq(f.groupID).And
	}

	if f.hasEventFilter {
		qFunc = qFunc("EventMetadata.UID").Eq(f.eventID).And
	}

	if f.hasStatusFilter {
		qFunc = qFunc("Status").In(badgerhold.Slice(f.status)...).And
	}

	if f.hasStartDateFilter {
		createdStart := primitive.NewDateTimeFromTime(time.Unix(f.searchParams.CreatedAtStart, 0))
		qFunc = qFunc("CreatedAt").Ge(createdStart).And
	}

	if f.hasEndDateFilter {
		createdEnd := primitive.NewDateTimeFromTime(time.Unix(f.searchParams.CreatedAtEnd, 0))
		qFunc = qFunc("CreatedAt").Le(createdEnd).And
	}

	// this is a play-safe workaround, uid will never be empty so use it to get the query object
	return qFunc("UID").Ne("")
}
