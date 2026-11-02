package event_deliveries

import (
	"errors"
	"time"
)

// readDashboardWindow keeps full UTC days on the rollup and reads only the
// partial boundary days live. A time filter must not inherit a whole day's
// total, and a long browser-local range must not fall back to a full live scan.
func readDashboardWindow[T any](start, end time.Time, rollup, live func(time.Time, time.Time) ([]T, error)) ([]T, bool, error) {
	start, end = start.UTC(), end.UTC()
	if end.Before(start) {
		return nil, false, errors.New("start date cannot be greater than end date")
	}
	fullStart, fullEnd := utcDate(start), utcDate(end)
	if fullStart.Before(start) {
		fullStart = fullStart.AddDate(0, 0, 1)
	}
	if !fullStart.Before(fullEnd) {
		rows, err := live(start, end)
		return rows, false, err
	}
	rows, err := rollup(fullStart, fullEnd)
	if err != nil {
		return nil, true, err
	}
	if start.Before(fullStart) {
		// PostgreSQL timestamps have microsecond precision. Keep the left
		// boundary disjoint from the first rolled-up UTC day.
		edge, err := live(start, fullStart.Add(-time.Microsecond))
		if err != nil {
			return nil, true, err
		}
		rows = append(rows, edge...)
	}
	// The live API includes end, including events exactly at UTC midnight.
	edge, err := live(fullEnd, end)
	if err != nil {
		return nil, true, err
	}
	return append(rows, edge...), true, nil
}
