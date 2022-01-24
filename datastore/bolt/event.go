package bolt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"go.etcd.io/bbolt"
)

var ErrInvalidPeriod = errors.New("specified data cannot be generated for period")

type eventRepo struct {
	db         *bbolt.DB
	bucketName string
}

func NewEventRepo(db *bbolt.DB) datastore.EventRepository {
	bucketName := "events"
	err := db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(bucketName))
		return err
	})

	if err != nil {
		return nil
	}

	return &eventRepo{db: db, bucketName: bucketName}
}

type TimeDuration struct {
	day   int
	month int
	year  int
}

func (e *eventRepo) CreateEvent(ctx context.Context, event *datastore.Event) error {
	return e.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(e.bucketName))

		evt, err := json.Marshal(event)
		if err != nil {
			return err
		}

		pErr := b.Put([]byte(event.UID), evt)
		if pErr != nil {
			return pErr
		}

		return nil
	})
}

func (e *eventRepo) CountGroupMessages(ctx context.Context, gid string) (int64, error) {
	count := int64(0)
	err := e.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(e.bucketName))

		return b.ForEach(func(k, v []byte) error {
			var event *datastore.Event
			err := json.Unmarshal(v, &event)
			if err != nil {
				return err
			}

			if event.AppMetadata.GroupID == gid {
				count++
			}

			return nil
		})
	})

	return count, err
}

func (e *eventRepo) DeleteGroupEvents(ctx context.Context, gid string) error {
	return e.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(e.bucketName))

		return b.ForEach(func(k, v []byte) error {
			var event *datastore.Event
			err := json.Unmarshal(v, &event)
			if err != nil {
				return err
			}

			if event.AppMetadata.GroupID == gid {
				err := b.Delete([]byte(event.UID))
				if err != nil {
					return err
				}
			}

			return nil
		})
	})
}

func (e *eventRepo) FindEventByID(ctx context.Context, eid string) (*datastore.Event, error) {
	var event *datastore.Event
	err := e.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(e.bucketName))

		eventBytes := b.Get([]byte(eid))
		if eventBytes == nil {
			return datastore.ErrEventNotFound
		}

		err := json.Unmarshal(eventBytes, &event)
		if err != nil {
			return err
		}

		return nil
	})

	return event, err
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

	err := e.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(e.bucketName))

		err := b.ForEach(func(k, v []byte) error {
			var event datastore.Event
			err := json.Unmarshal(v, &event)
			if err != nil {
				return err
			}

			if event.AppMetadata.GroupID == groupID &&
				event.DocumentStatus != datastore.DeletedDocumentStatus &&
				event.CreatedAt.Time().Unix() >= start &&
				event.CreatedAt.Time().Unix() <= end {
				format := event.CreatedAt.Time().Format(timeFormat)

				fmt.Printf("%v\n", format)

				if _, ok := eventsIntervalsMap[format]; ok {
					eventsIntervalsMap[format]++
				}
			}

			return nil
		})

		if err != nil {
			return err
		}

		for date, count := range eventsIntervalsMap {
			interval, err := getInterval(date, timeFormat, period)

			if err != nil {
				return err
			}

			if count > 0 {
				eventsIntervals = append(eventsIntervals, datastore.EventInterval{
					Data:  datastore.EventIntervalData{Interval: interval, Time: date},
					Count: uint64(count)})
			}
		}

		sort.SliceStable(eventsIntervals, func(i, j int) bool {
			return eventsIntervals[i].Data.Time < eventsIntervals[j].Data.Time
		})

		return nil
	})

	return eventsIntervals, err
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

func (e *eventRepo) LoadEventsPagedByAppId(ctx context.Context, appId string, searchParams datastore.SearchParams, pageable datastore.Pageable) ([]datastore.Event, datastore.PaginationData, error) {
	return []datastore.Event{}, datastore.PaginationData{}, nil
}

func (e *eventRepo) LoadEventsPaged(ctx context.Context, groupID string, appId string, searchParams datastore.SearchParams, pageable datastore.Pageable) ([]datastore.Event, datastore.PaginationData, error) {
	return []datastore.Event{}, datastore.PaginationData{}, nil
}
