-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';

-- The dashboard's Successful/Failed cards needed a status breakdown, which the
-- original rollup could not express, so they fell back to two live COUNT(*)
-- scans of event_deliveries. Existing rows are totals across statuses and
-- cannot be split retroactively, so the table is rebuilt empty and the backfill
-- restarted from the oldest live delivery.
--
-- Readers that only want a daily total keep working unchanged: they already
-- SUM(count) GROUP BY day, which now sums the per-status rows for that day.
DROP TABLE IF EXISTS convoy.event_delivery_daily_counts;

CREATE TABLE convoy.event_delivery_daily_counts (
    project_id TEXT NOT NULL,
    endpoint_id TEXT NOT NULL,
    day DATE NOT NULL,
    status TEXT NOT NULL,
    count BIGINT NOT NULL,
    PRIMARY KEY (project_id, day, endpoint_id, status)
);

UPDATE convoy.event_delivery_daily_counts_meta
SET next_day = NULL, completed_at = NULL, last_pruned_at = NULL
WHERE name = 'backfill';

RESET lock_timeout;
RESET statement_timeout;

-- +migrate Down
SET lock_timeout = '2s';
SET statement_timeout = '30s';

DROP TABLE IF EXISTS convoy.event_delivery_daily_counts;

CREATE TABLE convoy.event_delivery_daily_counts (
    project_id TEXT NOT NULL,
    endpoint_id TEXT NOT NULL,
    day DATE NOT NULL,
    count BIGINT NOT NULL,
    PRIMARY KEY (project_id, day, endpoint_id)
);

UPDATE convoy.event_delivery_daily_counts_meta
SET next_day = NULL, completed_at = NULL, last_pruned_at = NULL
WHERE name = 'backfill';

RESET lock_timeout;
RESET statement_timeout;
