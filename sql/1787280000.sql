-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';

-- Dashboard summary chart: UTC daily counts per project/endpoint.
-- The API aggregates these rows; it does not scan event_deliveries once
-- backfill has completed. Prometheus queue gauges use the snapshot tables
-- below, not this rollup.
CREATE TABLE IF NOT EXISTS convoy.event_delivery_daily_counts (
    project_id TEXT NOT NULL,
    endpoint_id TEXT NOT NULL,
    day DATE NOT NULL,
    count BIGINT NOT NULL,
    PRIMARY KEY (project_id, endpoint_id, day)
);

CREATE INDEX IF NOT EXISTS idx_event_delivery_daily_counts_project_day
    ON convoy.event_delivery_daily_counts (project_id, day);

CREATE TABLE IF NOT EXISTS convoy.event_delivery_daily_counts_meta (
    name TEXT PRIMARY KEY,
    next_day DATE,
    completed_at TIMESTAMPTZ
);

INSERT INTO convoy.event_delivery_daily_counts_meta (name, next_day, completed_at)
VALUES ('backfill', NULL, NULL)
ON CONFLICT (name) DO NOTHING;

-- Prometheus Collect reads the current generation. A locked worker writes
-- a new generation then swings the pointer. These are not materialized views.
CREATE TABLE IF NOT EXISTS convoy.metrics_snapshot_meta (
    name TEXT PRIMARY KEY,
    generation BIGINT NOT NULL DEFAULT 0,
    refreshed_at TIMESTAMPTZ
);

INSERT INTO convoy.metrics_snapshot_meta (name, generation)
VALUES
    ('event_queue', 0),
    ('event_queue_backlog', 0),
    ('event_delivery_queue', 0),
    ('event_endpoint_backlog', 0)
ON CONFLICT (name) DO NOTHING;

CREATE TABLE IF NOT EXISTS convoy.metrics_event_queue (
    generation BIGINT NOT NULL,
    project_id TEXT NOT NULL,
    source_id TEXT NOT NULL,
    total BIGINT NOT NULL,
    PRIMARY KEY (generation, project_id, source_id)
);

CREATE TABLE IF NOT EXISTS convoy.metrics_event_queue_backlog (
    generation BIGINT NOT NULL,
    project_id TEXT NOT NULL,
    source_id TEXT NOT NULL,
    age_seconds DOUBLE PRECISION NOT NULL,
    PRIMARY KEY (generation, project_id, source_id)
);

CREATE TABLE IF NOT EXISTS convoy.metrics_event_delivery_queue (
    generation BIGINT NOT NULL,
    project_id TEXT NOT NULL,
    project_name TEXT NOT NULL,
    endpoint_id TEXT NOT NULL,
    status TEXT NOT NULL,
    event_type TEXT NOT NULL,
    source_id TEXT NOT NULL,
    organisation_id TEXT NOT NULL,
    organisation_name TEXT NOT NULL,
    total BIGINT NOT NULL,
    PRIMARY KEY (generation, project_id, endpoint_id, status, event_type, source_id, organisation_id)
);

CREATE TABLE IF NOT EXISTS convoy.metrics_event_endpoint_backlog (
    generation BIGINT NOT NULL,
    project_id TEXT NOT NULL,
    source_id TEXT NOT NULL,
    endpoint_id TEXT NOT NULL,
    age_seconds DOUBLE PRECISION NOT NULL,
    PRIMARY KEY (generation, project_id, source_id, endpoint_id)
);

RESET lock_timeout;
RESET statement_timeout;

-- +migrate Down
SET lock_timeout = '2s';
SET statement_timeout = '30s';

DROP TABLE IF EXISTS convoy.metrics_event_endpoint_backlog;
DROP TABLE IF EXISTS convoy.metrics_event_delivery_queue;
DROP TABLE IF EXISTS convoy.metrics_event_queue_backlog;
DROP TABLE IF EXISTS convoy.metrics_event_queue;
DROP TABLE IF EXISTS convoy.metrics_snapshot_meta;
DROP TABLE IF EXISTS convoy.event_delivery_daily_counts;
DROP TABLE IF EXISTS convoy.event_delivery_daily_counts_meta;

RESET lock_timeout;
RESET statement_timeout;
