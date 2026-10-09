-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';

-- Days whose per-status rollup no longer agrees with the live table.
--
-- The rollup's day totals could never go stale: a retry updates a delivery in
-- place, so the number of deliveries a day holds is fixed once the day is over.
-- Splitting those totals by status changed that. Status stays mutable for as
-- long as the row lives, through automatic retries and through force resend or
-- batch retry of anything an operator can select, so a day that left the
-- refresh window with rows still in flight keeps whatever split it had.
--
-- Knowing which days moved cannot come from the rollup side: a refreshed_at
-- column says when a day was last written, not whether it still matches, and
-- answering that means reading the day's rows, which costs the same as
-- rewriting it. So the writers record it. The status updates insert the day
-- here in the same statement, and only for rows already past the refresh
-- window, which is why an ordinary delivery being retried within the hour
-- writes nothing.
--
-- Keyed by day alone because the refresh rewrites a day for every project at
-- once. marked_at is for operators reading a backlog, not for the drain, which
-- clears markers by day inside the transaction that rewrites them.
CREATE TABLE IF NOT EXISTS convoy.event_delivery_daily_counts_stale (
    day DATE PRIMARY KEY,
    marked_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- The last UTC day the recent-window refresh ran for. The window only ever
-- covers yesterday and today, so a worker that stops for longer than that while
-- the API keeps ingesting leaves the days in between with no rollup rows at all,
-- and nothing revisits them once the backfill is complete. Comparing this
-- against today tells a run which days it owes.
ALTER TABLE convoy.event_delivery_daily_counts_meta
    ADD COLUMN IF NOT EXISTS last_refreshed_day DATE;

RESET lock_timeout;
RESET statement_timeout;

-- +migrate Down
SET lock_timeout = '2s';
SET statement_timeout = '30s';

DROP TABLE IF EXISTS convoy.event_delivery_daily_counts_stale;

ALTER TABLE convoy.event_delivery_daily_counts_meta
    DROP COLUMN IF EXISTS last_refreshed_day;

RESET lock_timeout;
RESET statement_timeout;
