package event_deliveries

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/common"
)

const eventDailyCountsBackfillName = "events_backfill"

func refreshEventDailyCounts(ctx context.Context, tx pgx.Tx, startDay, endDay pgtype.Date) error {
	_, err := tx.Exec(ctx, `
		DELETE FROM convoy.event_daily_counts
		WHERE day >= $1 AND day < $2`, startDay, endDay)
	if err != nil {
		return fmt.Errorf("refresh event daily counts: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO convoy.event_daily_counts (project_id, day, count)
		SELECT
			e.project_id,
			(e.created_at AT TIME ZONE 'UTC')::date,
			COUNT(*)::bigint
		FROM convoy.events e
		WHERE e.deleted_at IS NULL
		  AND e.created_at >= ($1::timestamp AT TIME ZONE 'UTC')
		  AND e.created_at < ($2::timestamp AT TIME ZONE 'UTC')
		GROUP BY e.project_id, (e.created_at AT TIME ZONE 'UTC')::date
		ON CONFLICT (project_id, day)
		DO UPDATE SET count = EXCLUDED.count`, startDay, endDay)
	if err != nil {
		return fmt.Errorf("refresh event daily counts: %w", err)
	}

	_, err = tx.Exec(ctx, `
		DELETE FROM convoy.event_endpoint_daily_counts
		WHERE day >= $1 AND day < $2`, startDay, endDay)
	if err != nil {
		return fmt.Errorf("refresh event endpoint daily counts: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO convoy.event_endpoint_daily_counts (project_id, endpoint_id, day, count)
		SELECT
			e.project_id,
			ee.endpoint_id,
			(e.created_at AT TIME ZONE 'UTC')::date,
			COUNT(DISTINCT e.id)::bigint
		FROM convoy.events e
		JOIN convoy.events_endpoints ee ON ee.event_id = e.id
		WHERE e.deleted_at IS NULL
		  AND e.created_at >= ($1::timestamp AT TIME ZONE 'UTC')
		  AND e.created_at < ($2::timestamp AT TIME ZONE 'UTC')
		GROUP BY e.project_id, ee.endpoint_id, (e.created_at AT TIME ZONE 'UTC')::date
		ON CONFLICT (project_id, day, endpoint_id)
		DO UPDATE SET count = EXCLUDED.count`, startDay, endDay)
	if err != nil {
		return fmt.Errorf("refresh event endpoint daily counts: %w", err)
	}
	return nil
}

// RefreshEventDailyCounts replaces UTC event daily counts for [start, end).
// It does not touch delivery rollup rows or shared stale markers. The
// events_backfill walk uses this so a finished delivery backfill is not
// rewritten one historical day at a time.
func (s *Service) RefreshEventDailyCounts(ctx context.Context, start, end time.Time) error {
	startDay := pgtype.Date{Time: utcDate(start), Valid: true}
	endDay := pgtype.Date{Time: utcDate(end), Valid: true}
	if !endDay.Time.After(startDay.Time) {
		return nil
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("refresh event daily counts: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := refreshEventDailyCounts(ctx, tx, startDay, endDay); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("refresh event daily counts: %w", err)
	}
	return nil
}

// AdvanceEventDailyCountsBackfill walks one UTC day of event history.
// It is independent of the delivery backfill so an instance that already
// finished that walk still fills the events tables. It must not call
// RefreshDailyCounts: that rewrite also deletes and re-aggregates
// event_delivery_daily_counts.
func (s *Service) AdvanceEventDailyCountsBackfill(ctx context.Context) (bool, error) {
	var nextDay pgtype.Date
	var completedAt pgtype.Timestamptz
	err := s.db.QueryRow(ctx, `
		SELECT next_day, completed_at
		FROM convoy.event_delivery_daily_counts_meta
		WHERE name = $1`, eventDailyCountsBackfillName).Scan(&nextDay, &completedAt)
	if err != nil {
		return false, fmt.Errorf("load event daily counts backfill meta: %w", err)
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
			FROM convoy.events
			WHERE deleted_at IS NULL`).Scan(&earliest)
		if err != nil {
			return false, fmt.Errorf("earliest event day: %w", err)
		}
		if !earliest.Valid {
			return true, s.markEventDailyCountsBackfillCompleted(ctx)
		}
		day = earliest.Time
	}

	day = utcDate(day)
	if !day.Before(today) {
		return true, s.markEventDailyCountsBackfillCompleted(ctx)
	}

	if err := s.RefreshEventDailyCounts(ctx, day, day.AddDate(0, 0, 1)); err != nil {
		return false, err
	}

	next := day.AddDate(0, 0, 1)
	if !next.Before(today) {
		return true, s.markEventDailyCountsBackfillCompleted(ctx)
	}

	_, err = s.db.Exec(ctx, `
		UPDATE convoy.event_delivery_daily_counts_meta
		SET next_day = $1
		WHERE name = $2 AND completed_at IS NULL`, next, eventDailyCountsBackfillName)
	if err != nil {
		return false, fmt.Errorf("advance event daily counts backfill day: %w", err)
	}
	return false, nil
}

func (s *Service) eventDailyCountsBackfillCompleted(ctx context.Context) (bool, error) {
	var completed bool
	err := s.db.QueryRow(ctx, `
		SELECT completed_at IS NOT NULL
		FROM convoy.event_delivery_daily_counts_meta
		WHERE name = $1`, eventDailyCountsBackfillName).Scan(&completed)
	if err != nil {
		return false, err
	}
	return completed, nil
}

func (s *Service) markEventDailyCountsBackfillCompleted(ctx context.Context) error {
	_, err := s.db.Exec(ctx, `
		UPDATE convoy.event_delivery_daily_counts_meta
		SET completed_at = NOW(), next_day = NULL
		WHERE name = $1`, eventDailyCountsBackfillName)
	if err != nil {
		return fmt.Errorf("mark event daily counts backfill completed: %w", err)
	}
	s.logger.Info("event daily counts backfill complete")
	if err := s.PruneEventDailyCountsBeforeLiveHistory(ctx); err != nil {
		return err
	}
	return s.stampEventDailyCountsPruned(ctx)
}

func (s *Service) maybePruneEventDailyCountsBeforeLiveHistory(ctx context.Context) error {
	var lastPruned pgtype.Timestamptz
	err := s.db.QueryRow(ctx, `
		SELECT last_pruned_at
		FROM convoy.event_delivery_daily_counts_meta
		WHERE name = $1`, eventDailyCountsBackfillName).Scan(&lastPruned)
	if err != nil {
		return fmt.Errorf("load event daily counts prune meta: %w", err)
	}
	if lastPruned.Valid && time.Since(lastPruned.Time) < 24*time.Hour {
		return nil
	}

	var rollupMin pgtype.Date
	err = s.db.QueryRow(ctx, `
		SELECT MIN(day)
		FROM (
			SELECT MIN(day) AS day FROM convoy.event_daily_counts
			UNION ALL
			SELECT MIN(day) FROM convoy.event_endpoint_daily_counts
		) days`).Scan(&rollupMin)
	if err != nil {
		return fmt.Errorf("event rollup min day: %w", err)
	}
	if !rollupMin.Valid {
		return nil
	}
	recentCutoff := utcDate(time.Now()).AddDate(0, 0, -2)
	if !rollupMin.Time.Before(recentCutoff) {
		return nil
	}

	if err := s.PruneEventDailyCountsBeforeLiveHistory(ctx); err != nil {
		return err
	}
	return s.stampEventDailyCountsPruned(ctx)
}

func (s *Service) stampEventDailyCountsPruned(ctx context.Context) error {
	_, err := s.db.Exec(ctx, `
		UPDATE convoy.event_delivery_daily_counts_meta
		SET last_pruned_at = NOW()
		WHERE name = $1`, eventDailyCountsBackfillName)
	if err != nil {
		return fmt.Errorf("stamp event daily counts prune time: %w", err)
	}
	return nil
}

// PruneEventDailyCountsBeforeLiveHistory drops event rollup days older than
// the oldest live event. Retention removes those events; the recent rewrite
// does not.
func (s *Service) PruneEventDailyCountsBeforeLiveHistory(ctx context.Context) error {
	_, err := s.db.Exec(ctx, `
		DELETE FROM convoy.event_daily_counts
		WHERE day < COALESCE(
			(SELECT MIN((created_at AT TIME ZONE 'UTC')::date)
			 FROM convoy.events
			 WHERE deleted_at IS NULL),
			'infinity'::date)`)
	if err != nil {
		return fmt.Errorf("prune event daily counts: %w", err)
	}
	_, err = s.db.Exec(ctx, `
		DELETE FROM convoy.event_endpoint_daily_counts
		WHERE day < COALESCE(
			(SELECT MIN((created_at AT TIME ZONE 'UTC')::date)
			 FROM convoy.events
			 WHERE deleted_at IS NULL),
			'infinity'::date)`)
	if err != nil {
		return fmt.Errorf("prune event endpoint daily counts: %w", err)
	}
	return nil
}

// LoadEventIntervals is the dashboard chart and Events sent total. It counts
// events, not deliveries. A single-endpoint portal reads the endpoint rollup;
// more than one endpoint is a live distinct count so a shared event is not
// counted twice.
func (s *Service) LoadEventIntervals(ctx context.Context, projectID string, params datastore.SearchParams, period datastore.Period, endpointIDs []string) ([]datastore.EventInterval, error) {
	start, end := getCreatedDateFilter(params.CreatedAtStart, params.CreatedAtEnd)

	completed, err := s.eventDailyCountsBackfillCompleted(ctx)
	if err != nil {
		return nil, err
	}

	var rawRows []intervalRow
	switch {
	case len(endpointIDs) > 1:
		rawRows, err = s.loadEventIntervalsLive(ctx, projectID, start, end, period, endpointIDs)
	case completed && len(endpointIDs) == 1:
		rawRows, err = s.loadEventIntervalsFromEndpointRollup(ctx, projectID, start, end, period, endpointIDs[0])
	case completed:
		rawRows, err = s.loadEventIntervalsFromRollup(ctx, projectID, start, end, period)
	default:
		rawRows, err = s.loadEventIntervalsLive(ctx, projectID, start, end, period, endpointIDs)
	}
	if err != nil {
		return nil, err
	}

	intervals := make([]datastore.EventInterval, 0, len(rawRows))
	for _, r := range rawRows {
		intervals = append(intervals, datastore.EventInterval{
			Data: datastore.EventIntervalData{
				Interval: numericToInt64(r.DataIndex),
				Time:     common.PgTextToString(r.DataTotalTime),
			},
			Count: uint64(common.PgInt8ToInt64(r.Count)),
		})
	}

	if len(intervals) < minLen {
		var d time.Duration
		switch period {
		case datastore.Daily:
			d = time.Hour * 24
		case datastore.Weekly:
			d = time.Hour * 24 * 7
		case datastore.Monthly:
			d = time.Hour * 24 * 30
		case datastore.Yearly:
			d = time.Hour * 24 * 365
		}
		intervals, err = padIntervals(intervals, d, period)
		if err != nil {
			return nil, err
		}
	}

	return intervals, nil
}

func (s *Service) loadEventIntervalsFromRollup(ctx context.Context, projectID string, start, end time.Time, period datastore.Period) ([]intervalRow, error) {
	query, err := eventRollupIntervalQuery(period, false)
	if err != nil {
		return nil, err
	}
	return s.scanEventIntervalRows(ctx, query, projectID, start, end)
}

func (s *Service) loadEventIntervalsFromEndpointRollup(ctx context.Context, projectID string, start, end time.Time, period datastore.Period, endpointID string) ([]intervalRow, error) {
	query, err := eventRollupIntervalQuery(period, true)
	if err != nil {
		return nil, err
	}
	return s.scanEventIntervalRows(ctx, query, projectID, start, end, endpointID)
}

func (s *Service) loadEventIntervalsLive(ctx context.Context, projectID string, start, end time.Time, period datastore.Period, endpointIDs []string) ([]intervalRow, error) {
	query, err := eventLiveIntervalQuery(period, len(endpointIDs) > 0)
	if err != nil {
		return nil, err
	}
	if len(endpointIDs) > 0 {
		return s.scanEventIntervalRows(ctx, query, projectID, start, end, endpointIDs)
	}
	return s.scanEventIntervalRows(ctx, query, projectID, start, end)
}

func (s *Service) scanEventIntervalRows(ctx context.Context, query string, args ...any) ([]intervalRow, error) {
	rows, err := s.db.Query(ctx, query, args...)
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

func eventRollupIntervalQuery(period datastore.Period, byEndpoint bool) (string, error) {
	table := `convoy.event_daily_counts`
	filter := `
WHERE project_id = $1
  AND day >= ($2 AT TIME ZONE 'UTC')::date
  AND (day::timestamp AT TIME ZONE 'UTC') < $3`
	if byEndpoint {
		table = `convoy.event_endpoint_daily_counts`
		filter += `
  AND endpoint_id = $4`
	}

	return eventIntervalSelect(period, table, filter, "day")
}

func eventLiveIntervalQuery(period datastore.Period, scoped bool) (string, error) {
	from := `
FROM convoy.events e
WHERE e.project_id = $1
  AND e.deleted_at IS NULL
  AND e.created_at >= $2
  AND e.created_at <= $3`
	count := `COUNT(*)::bigint`
	if scoped {
		from = `
FROM convoy.events e
JOIN convoy.events_endpoints ee ON ee.event_id = e.id
WHERE e.project_id = $1
  AND e.deleted_at IS NULL
  AND e.created_at >= $2
  AND e.created_at <= $3
  AND ee.endpoint_id = ANY($4::TEXT[])`
		count = `COUNT(DISTINCT e.id)::bigint`
	}

	switch period {
	case datastore.Daily:
		return `
SELECT
    EXTRACT('doy' FROM e.created_at) AS "data.index",
    TO_CHAR(DATE_TRUNC('day', e.created_at), 'yyyy-mm-dd') AS "data.total_time",
    ` + count + ` AS count
` + from + `
GROUP BY DATE_TRUNC('day', e.created_at), EXTRACT('doy' FROM e.created_at)
ORDER BY DATE_TRUNC('day', e.created_at) ASC`, nil
	case datastore.Weekly:
		return `
SELECT
    EXTRACT('week' FROM DATE_TRUNC('week', e.created_at)) AS "data.index",
    TO_CHAR(DATE_TRUNC('week', e.created_at), 'yyyy-mm-dd') AS "data.total_time",
    ` + count + ` AS count
` + from + `
GROUP BY DATE_TRUNC('week', e.created_at)
ORDER BY DATE_TRUNC('week', e.created_at) ASC`, nil
	case datastore.Monthly:
		return `
SELECT
    EXTRACT('month' FROM DATE_TRUNC('month', e.created_at)) AS "data.index",
    TO_CHAR(DATE_TRUNC('month', e.created_at), 'yyyy-mm') AS "data.total_time",
    ` + count + ` AS count
` + from + `
GROUP BY DATE_TRUNC('month', e.created_at)
ORDER BY DATE_TRUNC('month', e.created_at) ASC`, nil
	case datastore.Yearly:
		return `
SELECT
    EXTRACT('year' FROM DATE_TRUNC('year', e.created_at)) AS "data.index",
    TO_CHAR(DATE_TRUNC('year', e.created_at), 'yyyy') AS "data.total_time",
    ` + count + ` AS count
` + from + `
GROUP BY DATE_TRUNC('year', e.created_at)
ORDER BY DATE_TRUNC('year', e.created_at) ASC`, nil
	default:
		return "", fmt.Errorf("specified data cannot be generated for period")
	}
}

func eventIntervalSelect(period datastore.Period, table, filter, dayExpr string) (string, error) {
	from := `
FROM ` + table + `
` + filter

	switch period {
	case datastore.Daily:
		return `
SELECT
    EXTRACT('doy' FROM ` + dayExpr + `) AS "data.index",
    TO_CHAR(` + dayExpr + `, 'yyyy-mm-dd') AS "data.total_time",
    SUM(count)::bigint AS count
` + from + `
GROUP BY ` + dayExpr + `
ORDER BY ` + dayExpr + ` ASC`, nil
	case datastore.Weekly:
		return `
SELECT
    EXTRACT('week' FROM DATE_TRUNC('week', ` + dayExpr + `::timestamp)) AS "data.index",
    TO_CHAR(DATE_TRUNC('week', ` + dayExpr + `::timestamp), 'yyyy-mm-dd') AS "data.total_time",
    SUM(count)::bigint AS count
` + from + `
GROUP BY DATE_TRUNC('week', ` + dayExpr + `::timestamp)
ORDER BY DATE_TRUNC('week', ` + dayExpr + `::timestamp) ASC`, nil
	case datastore.Monthly:
		return `
SELECT
    EXTRACT('month' FROM DATE_TRUNC('month', ` + dayExpr + `::timestamp)) AS "data.index",
    TO_CHAR(DATE_TRUNC('month', ` + dayExpr + `::timestamp), 'yyyy-mm') AS "data.total_time",
    SUM(count)::bigint AS count
` + from + `
GROUP BY DATE_TRUNC('month', ` + dayExpr + `::timestamp)
ORDER BY DATE_TRUNC('month', ` + dayExpr + `::timestamp) ASC`, nil
	case datastore.Yearly:
		return `
SELECT
    EXTRACT('year' FROM DATE_TRUNC('year', ` + dayExpr + `::timestamp)) AS "data.index",
    TO_CHAR(DATE_TRUNC('year', ` + dayExpr + `::timestamp), 'yyyy') AS "data.total_time",
    SUM(count)::bigint AS count
` + from + `
GROUP BY DATE_TRUNC('year', ` + dayExpr + `::timestamp)
ORDER BY DATE_TRUNC('year', ` + dayExpr + `::timestamp) ASC`, nil
	default:
		return "", fmt.Errorf("specified data cannot be generated for period")
	}
}
