package bolt

import (
	"context"
	"errors"
	"math"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/timshannon/badgerhold/v4"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
)

var ErrInvalidPeriod = errors.New("specified data cannot be generated for period")

type eventRepo struct {
	db *badgerhold.Store
}

func NewEventRepo(db *badgerhold.Store) datastore.EventRepository {
	return &eventRepo{db: db}
}

type TimeDuration struct {
	day   int
	month int
	year  int
}

func (e *eventRepo) CreateEvent(ctx context.Context, event *datastore.Event) error {
	return e.db.Insert(event.UID, event)
}

func (e *eventRepo) CountGroupMessages(ctx context.Context, gid string) (int64, error) {
	count, err := e.db.Count(&datastore.Event{}, badgerhold.Where("AppMetadata.GroupID").Eq(gid))

	return int64(count), err
}

func (e *eventRepo) DeleteGroupEvents(ctx context.Context, gid string) error {
	return e.db.DeleteMatching(&datastore.Event{}, badgerhold.Where("AppMetadata.GroupID").Eq(gid))
}

func (e *eventRepo) FindEventByID(ctx context.Context, eid string) (*datastore.Event, error) {
	var event datastore.Event
	err := e.db.Get(eid, &event)

	return &event, err
}

func (e *eventRepo) LoadEventIntervals(ctx context.Context, groupID string, searchParams datastore.SearchParams, period datastore.Period, interval int) ([]datastore.EventInterval, error) {
	eventsIntervals := make([]datastore.EventInterval, 0)
	eventsIntervalsMap := make(map[string]int)

	start := searchParams.CreatedAtStart
	end := searchParams.CreatedAtEnd

	startDay := time.Unix(start, 0)
	endDay := time.Unix(end, 0)

	if end <= 0 || end < start {
		endDay = startDay.Add(time.Hour * 23)
		end = endDay.Unix()
	}

	timeDur, timeFormat, fErr := getFormat(period)
	if fErr != nil {
		return nil, fErr
	}

	// set end date to the end of the year so the loop would count it
	if period == datastore.Yearly {
		endDay = endDay.Add(time.Hour * 23 * 365)
		end = endDay.Unix()
	}

	for i := startDay; i.Unix() <= endDay.Unix(); i = i.AddDate(timeDur.year, timeDur.month, timeDur.day) {
		if _, ok := eventsIntervalsMap[i.Format(timeFormat)]; !ok {
			eventsIntervalsMap[i.Format(timeFormat)] = 0
		}
	}

	//err := e.db.View(func(tx *bbolt.Tx) error {
	//	b := tx.Bucket([]byte(""))
	//
	//	err := b.ForEach(func(k, v []byte) error {
	//		var event datastore.Event
	//		err := json.Unmarshal(v, &event)
	//		if err != nil {
	//			return err
	//		}
	//
	//		if event.AppMetadata.GroupID == groupID &&
	//			event.DocumentStatus != datastore.DeletedDocumentStatus &&
	//			event.CreatedAt.Time().Unix() >= start &&
	//			event.CreatedAt.Time().Unix() <= end {
	//			format := event.CreatedAt.Time().Format(timeFormat)
	//
	//			if _, ok := eventsIntervalsMap[format]; ok {
	//				eventsIntervalsMap[format]++
	//			}
	//		}
	//
	//		return nil
	//	})
	//
	//	if err != nil {
	//		return err
	//	}
	//
	//	for date, count := range eventsIntervalsMap {
	//		interval, err := getInterval(date, timeFormat, period)
	//
	//		if err != nil {
	//			return err
	//		}
	//
	//		if count > 0 {
	//			eventsIntervals = append(eventsIntervals, datastore.EventInterval{
	//				Data:  datastore.EventIntervalData{Interval: interval, Time: date},
	//				Count: uint64(count)})
	//		}
	//	}
	//
	//	sort.SliceStable(eventsIntervals, func(i, j int) bool {
	//		return eventsIntervals[i].Data.Time < eventsIntervals[j].Data.Time
	//	})
	//
	//	return nil
	//})

	return eventsIntervals, nil
}

func getFormat(period datastore.Period) (TimeDuration, string, error) {
	var dailyIntervalFormat = "2006-01-02" // 1 day
	var weeklyIntervalFormat = "2006-01"   // 1 week
	var monthlyIntervalFormat = "2006-01"  // 1 month
	var yearlyIntervalFormat = "2006"      // 1 year

	var timeDur TimeDuration
	var timeFormat string
	switch period {
	case datastore.Daily:
		timeDur = TimeDuration{day: 1}
		timeFormat = dailyIntervalFormat
	case datastore.Weekly:
		timeDur = TimeDuration{day: 7}
		timeFormat = weeklyIntervalFormat
	case datastore.Monthly:
		timeDur = TimeDuration{month: 1}
		timeFormat = monthlyIntervalFormat
	case datastore.Yearly:
		timeDur = TimeDuration{year: 1}
		timeFormat = yearlyIntervalFormat
	default:
		return TimeDuration{}, "", ErrInvalidPeriod
	}

	return timeDur, timeFormat, nil
}

func getInterval(date, timeFormat string, period datastore.Period) (int64, error) {
	t, err := time.Parse(timeFormat, date)

	if err != nil {
		return 0, err
	}

	year, month, day := t.Date()
	_, week := t.ISOWeek()

	var interval int
	switch period {
	case datastore.Daily:
		interval = day
	case datastore.Weekly:
		interval = week
	case datastore.Monthly:
		interval = int(month)
	case datastore.Yearly:
		interval = year
	default:
		return 0, ErrInvalidPeriod
	}

	return int64(interval), nil
}

func (e *eventRepo) LoadEventsPaged(ctx context.Context, groupId string, appId string, searchParams datastore.SearchParams, pageable datastore.Pageable) ([]datastore.Event, datastore.PaginationData, error) {
	f := &filter{
		appID:        appId,
		groupID:      groupId,
		searchParams: searchParams,

		hasAppFilter:       !util.IsStringEmpty(appId),
		hasGroupFilter:     !util.IsStringEmpty(groupId),
		hasEndDateFilter:   searchParams.CreatedAtEnd > 0,
		hasStartDateFilter: searchParams.CreatedAtStart > 0,
	}

	if pageable.Page < 1 {
		pageable.Page = 1
	}

	if pageable.PerPage < 1 {
		pageable.PerPage = 10
	}

	prevPage := pageable.Page - 1
	lowerBound := pageable.PerPage * prevPage

	var events []datastore.Event
	var pg datastore.PaginationData

	q := e.generateQuery(f).Skip(lowerBound).Limit(pageable.PerPage)
	err := e.db.Find(&events, q)
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

	return events, pg, err
}

func (e *eventRepo) generateQuery(f *filter) *badgerhold.Query {
	qFunc := badgerhold.Where

	if f.hasAppFilter {
		qFunc = qFunc("AppMetadata.UID").Eq(f.appID).And
	}

	if f.hasGroupFilter {
		qFunc = qFunc("AppMetadata.GroupID").Eq(f.groupID).And
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
