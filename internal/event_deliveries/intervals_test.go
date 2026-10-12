package event_deliveries

import (
	"testing"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/stretchr/testify/require"
)

func TestPadIntervalsRequestedWindow(t *testing.T) {
	date := func(s string) time.Time {
		v, err := time.Parse("2006-01-02", s)
		require.NoError(t, err)
		return v
	}
	for _, tc := range []struct {
		name, start, end string
		period           datastore.Period
		want             []string
	}{
		{"historical single day", "2020-08-01", "2020-08-01", datastore.Daily, []string{"2020-08-01"}},
		{"daily gaps and leap day", "2024-02-28", "2024-03-01", datastore.Daily, []string{"2024-02-28", "2024-02-29", "2024-03-01"}},
		{"ISO weeks cross year", "2025-12-31", "2026-01-06", datastore.Weekly, []string{"2025-12-29", "2026-01-05"}},
		{"calendar months", "2024-01-31", "2024-04-01", datastore.Monthly, []string{"2024-01", "2024-02", "2024-03", "2024-04"}},
		{"calendar years", "2023-12-31", "2025-01-01", datastore.Yearly, []string{"2023", "2024", "2025"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows := []datastore.EventInterval{{Data: datastore.EventIntervalData{Time: tc.want[len(tc.want)-1], Interval: 7}, Count: 9}}
			padded, err := padIntervals(rows, date(tc.start), date(tc.end), tc.period)
			require.NoError(t, err)
			got := make([]string, len(padded))
			for i, row := range padded {
				got[i] = row.Data.Time
			}
			require.Equal(t, tc.want, got)
			require.Equal(t, rows[0], padded[len(padded)-1])
			empty, err := padIntervals(nil, date(tc.start), date(tc.end), tc.period)
			require.NoError(t, err)
			require.Len(t, empty, len(tc.want))
		})
	}
	rows := []datastore.EventInterval{{Data: datastore.EventIntervalData{Time: "2026-10-12"}, Count: 9}}
	padded, err := padIntervals(rows, date("1900-01-01"), date("2026-10-12"), datastore.Daily)
	require.NoError(t, err)
	require.Equal(t, rows, padded)
	_, err = padIntervals(nil, date("2026-10-12"), date("2026-10-11"), datastore.Daily)
	require.Error(t, err)
}
