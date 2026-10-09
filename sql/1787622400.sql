-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';

-- Events sent and the dashboard chart count events, not deliveries. The
-- delivery rollup cannot express that: one event can produce many delivery
-- rows. These tables are written by the same minute job, in the same
-- transaction, as event_delivery_daily_counts.
--
-- (project_id, day) is the project card. (project_id, endpoint_id, day) is a
-- single-endpoint portal. A portal with more than one endpoint reads live
-- distinct event ids so a shared event is not counted twice.
CREATE TABLE IF NOT EXISTS convoy.event_daily_counts (
    project_id TEXT NOT NULL,
    day DATE NOT NULL,
    count BIGINT NOT NULL,
    PRIMARY KEY (project_id, day)
);

CREATE TABLE IF NOT EXISTS convoy.event_endpoint_daily_counts (
    project_id TEXT NOT NULL,
    endpoint_id TEXT NOT NULL,
    day DATE NOT NULL,
    count BIGINT NOT NULL,
    PRIMARY KEY (project_id, day, endpoint_id)
);

-- Separate from 'backfill' so an instance that already finished the delivery
-- walk does not serve an empty events rollup, and so we do not rebuild
-- delivery history to introduce events.
INSERT INTO convoy.event_delivery_daily_counts_meta (name, next_day, completed_at)
VALUES ('events_backfill', NULL, NULL)
ON CONFLICT (name) DO NOTHING;

RESET lock_timeout;
RESET statement_timeout;

-- +migrate Down
SET lock_timeout = '2s';
SET statement_timeout = '30s';

DROP TABLE IF EXISTS convoy.event_endpoint_daily_counts;
DROP TABLE IF EXISTS convoy.event_daily_counts;

DELETE FROM convoy.event_delivery_daily_counts_meta
WHERE name = 'events_backfill';

RESET lock_timeout;
RESET statement_timeout;
