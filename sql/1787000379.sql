-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';

-- Pause state for the Postgres queue driver. Redis carries this inside asynq;
-- postgres needs somewhere to put it, and it has to be in the database rather
-- than in a worker's memory so a pause applies to every replica at once.
-- A row exists only for a queue an operator has touched: absent means running.
CREATE TABLE IF NOT EXISTS convoy.queue_state (
    queue_name  TEXT PRIMARY KEY,
    paused_at   TIMESTAMPTZ,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Daily throughput per queue. The driver deletes a row when its task succeeds,
-- so without this table a finished task leaves no trace and "how much did this
-- queue do today" has no answer. One row per queue per UTC day keeps the
-- write a single upsert on a row already in cache.
--
-- failed counts attempts, not tasks: a delivery that fails twice before
-- succeeding contributes two failures and one processed, which is what asynq
-- records and what makes the error rate mean "share of attempts that failed".
CREATE TABLE IF NOT EXISTS convoy.queue_job_stats (
    queue_name  TEXT NOT NULL,
    day         DATE NOT NULL,
    processed   BIGINT NOT NULL DEFAULT 0,
    failed      BIGINT NOT NULL DEFAULT 0,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (queue_name, day)
);

-- Registered periodic tasks. The Postgres scheduler keeps its entries in Go
-- memory, in the agent process, so the API has no way to read them; this is
-- where the scheduler publishes what it registered and when it last fired.
-- Rows are owned by whichever replica registered them: the spec is identical
-- across replicas by construction, so last write wins is not a conflict.
CREATE TABLE IF NOT EXISTS convoy.queue_scheduler_entries (
    id           TEXT PRIMARY KEY,
    task_name    TEXT NOT NULL,
    queue_name   TEXT NOT NULL,
    spec         TEXT NOT NULL,
    next_run_at  TIMESTAMPTZ,
    prev_run_at  TIMESTAMPTZ,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

RESET lock_timeout;
RESET statement_timeout;

-- +migrate Down
SET lock_timeout = '2s';
SET statement_timeout = '30s';

DROP TABLE IF EXISTS convoy.queue_scheduler_entries;
DROP TABLE IF EXISTS convoy.queue_job_stats;
DROP TABLE IF EXISTS convoy.queue_state;

RESET lock_timeout;
RESET statement_timeout;
