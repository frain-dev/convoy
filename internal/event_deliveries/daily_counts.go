package event_deliveries

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/frain-dev/convoy/datastore"
)

const dailyCountsBackfillName = "backfill"

// RefreshDailyCounts replaces UTC daily counts for [start, end).
func (s *Service) RefreshDailyCounts(ctx context.Context, start, end time.Time) error {
	startDay := utcDate(start)
	endDay := utcDate(end)
	if !endDay.After(startDay) {
		return nil
	}

	_, err := s.db.Exec(ctx, `
		WITH bounds AS (
			SELECT $1::date AS start_day, $2::date AS end_day
		),
		deleted AS (
			DELETE FROM convoy.event_delivery_daily_counts d
			USING bounds b
			WHERE d.day >= b.start_day AND d.day < b.end_day
			RETURNING 1
		),
		aggregated AS (
			SELECT
				ed.project_id,
				COALESCE(ed.endpoint_id, '') AS endpoint_id,
				(ed.created_at AT TIME ZONE 'UTC')::date AS day,
				COUNT(*)::bigint AS count
			FROM convoy.event_deliveries ed, bounds b
			WHERE ed.deleted_at IS NULL
			  AND ed.created_at >= (b.start_day::timestamp AT TIME ZONE 'UTC')
			  AND ed.created_at < (b.end_day::timestamp AT TIME ZONE 'UTC')
			GROUP BY ed.project_id, COALESCE(ed.endpoint_id, ''), (ed.created_at AT TIME ZONE 'UTC')::date
		)
		INSERT INTO convoy.event_delivery_daily_counts (project_id, endpoint_id, day, count)
		SELECT project_id, endpoint_id, day, count FROM aggregated`,
		startDay, endDay)
	if err != nil {
		return fmt.Errorf("refresh event delivery daily counts: %w", err)
	}
	return nil
}

// RefreshRecentDailyCounts rewrites yesterday and today in UTC.
func (s *Service) RefreshRecentDailyCounts(ctx context.Context) error {
	today := utcDate(time.Now())
	return s.RefreshDailyCounts(ctx, today.AddDate(0, 0, -1), today.AddDate(0, 0, 1))
}

// AdvanceDailyCountsBackfill refreshes one UTC day of history. Returns
// whether the backfill is complete after this step.
func (s *Service) AdvanceDailyCountsBackfill(ctx context.Context) (bool, error) {
	var nextDay pgtype.Date
	var completedAt pgtype.Timestamptz
	err := s.db.QueryRow(ctx, `
		SELECT next_day, completed_at
		FROM convoy.event_delivery_daily_counts_meta
		WHERE name = $1`, dailyCountsBackfillName).Scan(&nextDay, &completedAt)
	if err != nil {
		return false, fmt.Errorf("load daily counts backfill meta: %w", err)
	}
	if completedAt.Valid {
		return true, nil
	}

	today := utcDate(time.Now())
	day := nextDay.Time
	if !nextDay.Valid {
		var earliest pgtype.Date
		err = s.db.QueryRow(ctx, `
			SELECT MIN((created_at AT TIME ZONE 'UTC')::date)
			FROM convoy.event_deliveries
			WHERE deleted_at IS NULL`).Scan(&earliest)
		if err != nil {
			return false, fmt.Errorf("earliest event delivery day: %w", err)
		}
		if !earliest.Valid {
			return true, s.markDailyCountsBackfillCompleted(ctx)
		}
		day = earliest.Time
	}

	day = utcDate(day)
	if !day.Before(today) {
		return true, s.markDailyCountsBackfillCompleted(ctx)
	}

	if err := s.RefreshDailyCounts(ctx, day, day.AddDate(0, 0, 1)); err != nil {
		return false, err
	}

	next := day.AddDate(0, 0, 1)
	if !next.Before(today) {
		return true, s.markDailyCountsBackfillCompleted(ctx)
	}

	_, err = s.db.Exec(ctx, `
		UPDATE convoy.event_delivery_daily_counts_meta
		SET next_day = $1
		WHERE name = $2 AND completed_at IS NULL`, next, dailyCountsBackfillName)
	if err != nil {
		return false, fmt.Errorf("advance daily counts backfill day: %w", err)
	}
	return false, nil
}

func (s *Service) dailyCountsBackfillCompleted(ctx context.Context) (bool, error) {
	var completed bool
	err := s.db.QueryRow(ctx, `
		SELECT completed_at IS NOT NULL
		FROM convoy.event_delivery_daily_counts_meta
		WHERE name = $1`, dailyCountsBackfillName).Scan(&completed)
	if err != nil {
		return false, err
	}
	return completed, nil
}

func (s *Service) markDailyCountsBackfillCompleted(ctx context.Context) error {
	_, err := s.db.Exec(ctx, `
		UPDATE convoy.event_delivery_daily_counts_meta
		SET completed_at = NOW(), next_day = NULL
		WHERE name = $1`, dailyCountsBackfillName)
	if err != nil {
		return fmt.Errorf("mark daily counts backfill completed: %w", err)
	}
	return nil
}

func (s *Service) loadEventDeliveriesIntervalsFromRollup(ctx context.Context, projectID string, start, end time.Time, period datastore.Period, endpointIDs []string) ([]intervalRow, error) {
	query, err := rollupIntervalQuery(period)
	if err != nil {
		return nil, err
	}

	rows, err := s.db.Query(ctx, query, projectID, start, end, len(endpointIDs) > 0, endpointIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rawRows []intervalRow
	for rows.Next() {
		var r intervalRow
		if scanErr := rows.Scan(&r.DataIndex, &r.DataTotalTime, &r.Count); scanErr != nil {
			return nil, scanErr
		}
		rawRows = append(rawRows, r)
	}
	return rawRows, rows.Err()
}

func rollupIntervalQuery(period datastore.Period) (string, error) {
	// End is timestamp-exclusive at day grain: include day D iff D's UTC
	// midnight is < end. That matches live `created_at <= end` when the API
	// sends midnight (the goldie / dashboard contract) without pulling in the
	// rest of that calendar day. Intra-day end still includes D (day grain).
	const filter = `
FROM convoy.event_delivery_daily_counts
WHERE project_id = $1
  AND day >= ($2 AT TIME ZONE 'UTC')::date
  AND (day::timestamp AT TIME ZONE 'UTC') < $3
  AND (CASE WHEN $4::BOOLEAN THEN endpoint_id = ANY($5::TEXT[]) ELSE true END)`

	switch period {
	case datastore.Daily:
		return `
SELECT
    EXTRACT('doy' FROM day) AS "data.index",
    TO_CHAR(day, 'yyyy-mm-dd') AS "data.total_time",
    SUM(count)::bigint AS count
` + filter + `
GROUP BY day
ORDER BY day ASC`, nil
	case datastore.Weekly:
		return `
SELECT
    EXTRACT('week' FROM DATE_TRUNC('week', day::timestamp AT TIME ZONE 'UTC')) AS "data.index",
    TO_CHAR(DATE_TRUNC('week', day::timestamp AT TIME ZONE 'UTC'), 'yyyy-mm-dd') AS "data.total_time",
    SUM(count)::bigint AS count
` + filter + `
GROUP BY DATE_TRUNC('week', day::timestamp AT TIME ZONE 'UTC')
ORDER BY DATE_TRUNC('week', day::timestamp AT TIME ZONE 'UTC') ASC`, nil
	case datastore.Monthly:
		return `
SELECT
    EXTRACT('month' FROM DATE_TRUNC('month', day::timestamp AT TIME ZONE 'UTC')) AS "data.index",
    TO_CHAR(DATE_TRUNC('month', day::timestamp AT TIME ZONE 'UTC'), 'yyyy-mm') AS "data.total_time",
    SUM(count)::bigint AS count
` + filter + `
GROUP BY DATE_TRUNC('month', day::timestamp AT TIME ZONE 'UTC')
ORDER BY DATE_TRUNC('month', day::timestamp AT TIME ZONE 'UTC') ASC`, nil
	case datastore.Yearly:
		return `
SELECT
    EXTRACT('year' FROM DATE_TRUNC('year', day::timestamp AT TIME ZONE 'UTC')) AS "data.index",
    TO_CHAR(DATE_TRUNC('year', day::timestamp AT TIME ZONE 'UTC'), 'yyyy') AS "data.total_time",
    SUM(count)::bigint AS count
` + filter + `
GROUP BY DATE_TRUNC('year', day::timestamp AT TIME ZONE 'UTC')
ORDER BY DATE_TRUNC('year', day::timestamp AT TIME ZONE 'UTC') ASC`, nil
	default:
		return "", fmt.Errorf("specified data cannot be generated for period")
	}
}

func utcDate(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// intervalRow is the common shape for live and rollup interval queries.
type intervalRow struct {
	DataIndex     pgtype.Numeric
	DataTotalTime pgtype.Text
	Count         pgtype.Int8
}
