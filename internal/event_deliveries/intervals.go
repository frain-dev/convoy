package event_deliveries

import (
	"errors"
	"sort"
	"time"

	"github.com/frain-dev/convoy/datastore"
)

// padIntervals fills missing UTC calendar buckets in the requested window.
// Monthly/yearly steps use calendar arithmetic rather than 30/365-day approximations.
func padIntervals(intervals []datastore.EventInterval, start, end time.Time, period datastore.Period) ([]datastore.EventInterval, error) {
	start, end = start.UTC(), end.UTC()
	if end.Before(start) {
		return nil, errors.New("start date cannot be greater than end date")
	}
	var layout string
	var step func(time.Time) time.Time
	bucket := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	switch period {
	case datastore.Daily:
		layout, step = "2006-01-02", func(t time.Time) time.Time { return t.AddDate(0, 0, 1) }
	case datastore.Weekly:
		bucket = bucket.AddDate(0, 0, -(int(bucket.Weekday())+6)%7)
		layout, step = "2006-01-02", func(t time.Time) time.Time { return t.AddDate(0, 0, 7) }
	case datastore.Monthly:
		bucket = time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, time.UTC)
		layout, step = "2006-01", func(t time.Time) time.Time { return t.AddDate(0, 1, 0) }
	case datastore.Yearly:
		bucket = time.Date(start.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
		layout, step = "2006", func(t time.Time) time.Time { return t.AddDate(1, 0, 0) }
	default:
		return nil, errors.New("specified data cannot be generated for period")
	}
	byDate := make(map[string]datastore.EventInterval, len(intervals))
	for _, interval := range intervals {
		interval.Count += byDate[interval.Data.Time].Count
		byDate[interval.Data.Time] = interval
	}
	padded := make([]datastore.EventInterval, 0)
	for !bucket.After(end) {
		// Very large API windows keep their sparse result; do not allocate an
		// unbounded number of synthetic zeroes for a user-controlled date range.
		if len(padded) == 4096 {
			merged := make([]datastore.EventInterval, 0, len(byDate))
			for _, interval := range byDate {
				merged = append(merged, interval)
			}
			sort.Slice(merged, func(i, j int) bool { return merged[i].Data.Time < merged[j].Data.Time })
			return merged, nil
		}
		date := bucket.Format(layout)
		interval, found := byDate[date]
		if !found {
			interval.Data.Time = date
		}
		padded = append(padded, interval)
		bucket = step(bucket)
	}
	return padded, nil
}
