-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';

-- Partitioning a table rewrites it, which takes minutes to hours on a large
-- instance and holds locks while it runs. Until now the only report was the exit
-- code of a CLI invocation, so an operator watching a pod had no way to tell a
-- long phase from a hung one, and nothing survived the process at all.
--
-- Rows here are written by the server, not by the partition DDL. The DDL runs as
-- a single statement, so anything it wrote to this table would stay invisible to
-- other sessions until the whole conversion committed, which is exactly the
-- window that needs reporting. The server records progress from the notice
-- stream on a separate connection instead.
CREATE TABLE IF NOT EXISTS convoy.partition_runs (
    id            CHAR(26) PRIMARY KEY,
    table_name    TEXT NOT NULL,
    operation     TEXT NOT NULL,
    status        TEXT NOT NULL,
    phase         TEXT,
    -- Every step the conversion reported, in order, each with the time it
    -- arrived. phase is the newest of these, kept as a column because the list
    -- view reads it on its own; both are written by the same statement, so they
    -- cannot disagree. The list is capped in that statement: a conversion
    -- reports on the order of ten steps, and a row is not the place to
    -- accumulate an unbounded stream.
    steps         JSONB NOT NULL DEFAULT '[]'::JSONB,
    notice_count  BIGINT NOT NULL DEFAULT 0,
    error         TEXT,
    triggered_by  TEXT NOT NULL,
    started_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at  TIMESTAMPTZ,

    CONSTRAINT partition_runs_operation_check CHECK (operation IN ('partition', 'unpartition')),
    CONSTRAINT partition_runs_status_check CHECK (status IN ('running', 'completed', 'failed'))
);

-- One conversion at a time for the whole instance, which is what the CLI already
-- does when it is given no table name. Each conversion rewrites a table and
-- saturates disk doing it, so overlapping two makes both slower and holds two
-- sets of locks at once. A second start fails on this index instead of queueing
-- behind a multi-hour operation.
--
-- A run left behind by a killed pod keeps the instance blocked until an operator
-- resolves it. That is deliberate: the server cannot tell a crashed conversion
-- from one still running on another pod, and guessing wrong starts a second
-- rewrite of a table the first is still holding.
CREATE UNIQUE INDEX IF NOT EXISTS idx_partition_runs_single_active
    ON convoy.partition_runs ((status))
    WHERE status = 'running';

-- The UI lists most recent first.
CREATE INDEX IF NOT EXISTS idx_partition_runs_started_at
    ON convoy.partition_runs (started_at DESC);

RESET lock_timeout;
RESET statement_timeout;

-- +migrate Down
SET lock_timeout = '2s';
SET statement_timeout = '30s';

DROP TABLE IF EXISTS convoy.partition_runs;

RESET lock_timeout;
RESET statement_timeout;
