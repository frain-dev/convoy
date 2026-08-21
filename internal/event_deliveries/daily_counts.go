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
	startDay := pgtype.Date{Time: utcDate(start), Valid: true}
	endDay := pgtype.Date{Time: utcDate(end), Valid: true}
	if !endDay.Time.After(startDay.Time) {
		return nil
	}

	// The delete and the insert must be separate statements. Sub-statements of
	// one data-modifying CTE share a snapshot, so an insert of the same keys
	// cannot see the delete and collides on the primary key.
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("refresh event delivery daily counts: %w", err)
	}
	defer tx.Rollback(ctx)

	// Reaps keys that no longer aggregate to anything, which the upsert below
	// would otherwise leave at their stale count.
	_, err = tx.Exec(ctx, `
		DELETE FROM convoy.event_delivery_daily_counts
		WHERE day >= $1 AND day < $2`, startDay, endDay)
	if err != nil {
		return fmt.Errorf("refresh event delivery daily counts: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO convoy.event_delivery_daily_counts (project_id, endpoint_id, day, status, count)
		SELECT
			ed.project_id,
			COALESCE(ed.endpoint_id, ''),
			(ed.created_at AT TIME ZONE 'UTC')::date,
			ed.status,
			COUNT(*)::bigint
		FROM convoy.event_deliveries ed
		WHERE ed.deleted_at IS NULL
		  AND ed.created_at >= ($1::timestamp AT TIME ZONE 'UTC')
		  AND ed.created_at < ($2::timestamp AT TIME ZONE 'UTC')
		GROUP BY ed.project_id, COALESCE(ed.endpoint_id, ''), (ed.created_at AT TIME ZONE 'UTC')::date, ed.status
		ON CONFLICT (project_id, day, endpoint_id, status)
		DO UPDATE SET count = EXCLUDED.count`, startDay, endDay)
	if err != nil {
		return fmt.Errorf("refresh event delivery daily counts: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("refresh event delivery daily counts: %w", err)
	}
	return nil
}

// RefreshRecentDailyCounts rewrites yesterday and today in UTC. After
// backfill completes it may prune stale rollup days, but at most once per
// day and only when the rollup still holds history older than the recent
// refresh window.
func (s *Service) RefreshRecentDailyCounts(ctx context.Context) error {
	today := utcDate(time.Now())
	if err := s.RefreshDailyCounts(ctx, today.AddDate(0, 0, -1), today.AddDate(0, 0, 1)); err != nil {
		return err
	}
	completed, err := s.dailyCountsBackfillCompleted(ctx)
	if err != nil {
		return err
	}
	if completed {
		return s.maybePruneDailyCountsBeforeLiveHistory(ctx)
	}
	return nil
}

// PruneDailyCountsBeforeLiveHistory drops rollup days older than the
// oldest remaining live event delivery. Partition retention removes
// those deliveries; the recent rewrite does not, so the dashboard would
// keep counting purged history without this delete.
func (s *Service) PruneDailyCountsBeforeLiveHistory(ctx context.Context) error {
	// Parent-table scan: valid on heap and partitioned event_deliveries.
	_, err := s.db.Exec(ctx, `
		DELETE FROM convoy.event_delivery_daily_counts
		WHERE day < COALESCE(
			(SELECT MIN((created_at AT TIME ZONE 'UTC')::date)
			 FROM convoy.event_deliveries
			 WHERE deleted_at IS NULL),
			'infinity'::date)`)
	if err != nil {
		return fmt.Errorf("prune event delivery daily counts: %w", err)
	}
	return nil
}

func (s *Service) maybePruneDailyCountsBeforeLiveHistory(ctx context.Context) error {
	var lastPruned pgtype.Timestamptz
	err := s.db.QueryRow(ctx, `
		SELECT last_pruned_at
		FROM convoy.event_delivery_daily_counts_meta
		WHERE name = $1`, dailyCountsBackfillName).Scan(&lastPruned)
	if err != nil {
		return fmt.Errorf("load daily counts prune meta: %w", err)
	}
	if lastPruned.Valid && time.Since(lastPruned.Time) < 24*time.Hour {
		return nil
	}

	var rollupMin pgtype.Date
	err = s.db.QueryRow(ctx, `
		SELECT MIN(day) FROM convoy.event_delivery_daily_counts`).Scan(&rollupMin)
	if err != nil {
		return fmt.Errorf("rollup min day: %w", err)
	}
	if !rollupMin.Valid {
		return nil
	}
	recentCutoff := utcDate(time.Now()).AddDate(0, 0, -2)
	if !rollupMin.Time.Before(recentCutoff) {
		return nil
	}

	if err := s.PruneDailyCountsBeforeLiveHistory(ctx); err != nil {
		return err
	}
	return s.stampDailyCountsPruned(ctx)
}

func (s *Service) stampDailyCountsPruned(ctx context.Context) error {
	_, err := s.db.Exec(ctx, `
		UPDATE convoy.event_delivery_daily_counts_meta
		SET last_pruned_at = NOW()
		WHERE name = $1`, dailyCountsBackfillName)
	if err != nil {
		return fmt.Errorf("stamp daily counts prune time: %w", err)
	}
	return nil
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
	s.logger.Info("event delivery daily counts backfill complete")
	if err := s.PruneDailyCountsBeforeLiveHistory(ctx); err != nil {
		return err
	}
	return s.stampDailyCountsPruned(ctx)
}

// StatusTotalsSource names the table a StatusTotals answer came from, so a
// caller (or a curl) can tell a rollup read from a live scan.
type StatusTotalsSource string

const (
	StatusTotalsFromRollup StatusTotalsSource = "rollup"
	StatusTotalsFromLive   StatusTotalsSource = "live"
)

// Day grain, so the rollup and the chart describe the same window. End is
// timestamp-exclusive at day grain for the same reason as rollupIntervalQuery.
const statusTotalsRollupQuery = `
SELECT status, SUM(count)::bigint
FROM convoy.event_delivery_daily_counts
WHERE project_id = $1
  AND day >= ($2 AT TIME ZONE 'UTC')::date
  AND (day::timestamp AT TIME ZONE 'UTC') < $3
  AND (CASE WHEN $4::BOOLEAN THEN endpoint_id = ANY($5::TEXT[]) ELSE true END)
GROUP BY status`

// One grouped scan, where the dashboard previously issued one COUNT(*) per
// status and rendered a zero when either timed out.
const statusTotalsLiveQuery = `
SELECT status, COUNT(*)::bigint
FROM convoy.event_deliveries
WHERE project_id = $1
  AND deleted_at IS NULL
  AND created_at >= $2
  AND created_at <= $3
  AND (CASE WHEN $4::BOOLEAN THEN endpoint_id = ANY($5::TEXT[]) ELSE true END)
GROUP BY status`

// StatusTotals returns per-status delivery totals for the window, keyed by
// status, plus the table it read.
//
// It uses the rollup only once the backfill has completed, the same gate
// LoadEventDeliveriesIntervals applies, so the summary cards and the chart above
// them can never describe different sources. Statuses with no deliveries are
// absent from the map rather than present as zero: the caller distinguishes "no
// deliveries" from "we could not find out", which is the whole point of serving
// this instead of a swallowed error.
func (s *Service) StatusTotals(ctx context.Context, projectID string, params datastore.SearchParams,
	endpointIDs []string) (map[datastore.EventDeliveryStatus]int64, StatusTotalsSource, error) {
	start, end := getCreatedDateFilter(params.CreatedAtStart, params.CreatedAtEnd)

	completed, err := s.dailyCountsBackfillCompleted(ctx)
	if err != nil {
		return nil, "", err
	}

	query, source := statusTotalsLiveQuery, StatusTotalsFromLive
	if completed {
		query, source = statusTotalsRollupQuery, StatusTotalsFromRollup
	}

	rows, err := s.db.Query(ctx, query, projectID, start, end, len(endpointIDs) > 0, endpointIDs)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	totals := make(map[datastore.EventDeliveryStatus]int64)
	for rows.Next() {
		var status string
		var count int64
		if scanErr := rows.Scan(&status, &count); scanErr != nil {
			return nil, "", scanErr
		}
		totals[datastore.EventDeliveryStatus(status)] = count
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, "", rowsErr
	}
	return totals, source, nil
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
    EXTRACT('week' FROM DATE_TRUNC('week', day::timestamp)) AS "data.index",
    TO_CHAR(DATE_TRUNC('week', day::timestamp), 'yyyy-mm-dd') AS "data.total_time",
    SUM(count)::bigint AS count
` + filter + `
GROUP BY DATE_TRUNC('week', day::timestamp)
ORDER BY DATE_TRUNC('week', day::timestamp) ASC`, nil
	case datastore.Monthly:
		return `
SELECT
    EXTRACT('month' FROM DATE_TRUNC('month', day::timestamp)) AS "data.index",
    TO_CHAR(DATE_TRUNC('month', day::timestamp), 'yyyy-mm') AS "data.total_time",
    SUM(count)::bigint AS count
` + filter + `
GROUP BY DATE_TRUNC('month', day::timestamp)
ORDER BY DATE_TRUNC('month', day::timestamp) ASC`, nil
	case datastore.Yearly:
		return `
SELECT
    EXTRACT('year' FROM DATE_TRUNC('year', day::timestamp)) AS "data.index",
    TO_CHAR(DATE_TRUNC('year', day::timestamp), 'yyyy') AS "data.total_time",
    SUM(count)::bigint AS count
` + filter + `
GROUP BY DATE_TRUNC('year', day::timestamp)
ORDER BY DATE_TRUNC('year', day::timestamp) ASC`, nil
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
