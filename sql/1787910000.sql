-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';

-- Nightly partition retention drops are invisible in the admin UI today: only
-- partition conversions and index rebuilds are recorded in partition_runs.
-- One row here per retention Perform so operators can confirm what partman
-- dropped without querying Postgres.
CREATE TABLE IF NOT EXISTS convoy.retention_runs (
    id                CHAR(26) PRIMARY KEY,
    status            TEXT NOT NULL,
    retention_period  TEXT NOT NULL,
    details           JSONB NOT NULL DEFAULT '[]'::JSONB,
    error             TEXT,
    started_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at      TIMESTAMPTZ,

    CONSTRAINT retention_runs_status_check CHECK (status IN ('running', 'completed', 'failed', 'skipped'))
);

-- The UI lists most recent first. Blocking build is fine here: the table is
-- new and empty at migrate time, and this is not a partitioned parent (where
-- CONCURRENTLY is illegal anyway).
CREATE INDEX IF NOT EXISTS idx_retention_runs_started_at
    ON convoy.retention_runs (started_at DESC);

RESET lock_timeout;
RESET statement_timeout;

-- +migrate Down
SET lock_timeout = '2s';
SET statement_timeout = '30s';

DROP TABLE IF EXISTS convoy.retention_runs;

RESET lock_timeout;
RESET statement_timeout;
