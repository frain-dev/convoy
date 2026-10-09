-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';

-- Broker rows for the Postgres queue.Queuer driver. One row is one asynq task:
-- Write inserts it, workers claim with FOR UPDATE SKIP LOCKED, and a successful
-- handler deletes the row. Finished cron rows stay as tombstones past the
-- daily cleanup so same-tick replica enqueues stay idempotent. Duplicate
-- non-cron task IDs overwrite (same as asynq delete + re-enqueue).
-- convoy.jobs is a different table (backup/export jobs).
CREATE TABLE IF NOT EXISTS convoy.queue_jobs (
    id           TEXT PRIMARY KEY,
    task_name    TEXT NOT NULL,
    queue_name   TEXT NOT NULL,
    payload      BYTEA NOT NULL,
    headers      JSONB NOT NULL DEFAULT '{}'::JSONB,
    max_retry    INTEGER NOT NULL DEFAULT 25,
    retry_count  INTEGER NOT NULL DEFAULT 0,
    status       TEXT NOT NULL,
    run_at       TIMESTAMPTZ NOT NULL,
    claimed_at   TIMESTAMPTZ,
    last_error   TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT queue_jobs_status_check CHECK (status IN ('pending', 'processing', 'archived', 'completed'))
);

-- Postgres cache (queue_provider=postgres). TTL is expires_at; NULL means no expiry.
CREATE TABLE IF NOT EXISTS convoy.kv_cache (
    key        TEXT PRIMARY KEY,
    value      BYTEA NOT NULL,
    expires_at TIMESTAMPTZ
);

-- Postgres token-bucket limiter (queue_provider=postgres).
CREATE TABLE IF NOT EXISTS convoy.rate_limits (
    key        TEXT PRIMARY KEY,
    tokens     DOUBLE PRECISION NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

-- Cloud trial daily cap. One row per org avoids accumulating a row per day;
-- the atomic upsert resets event_count when the UTC day changes.
CREATE TABLE IF NOT EXISTS convoy.trial_event_counters (
    org_id      TEXT PRIMARY KEY,
    day         DATE NOT NULL,
    event_count BIGINT NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL
);

-- Atomic verification-email resend cooldown for Postgres broker mode. One row
-- per user is replaced after expiry; token-matched delete prevents a late
-- release from clearing a newer claim.
CREATE TABLE IF NOT EXISTS convoy.email_verification_resend_claims (
    user_uid   TEXT PRIMARY KEY,
    token      TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS convoy.batch_retry_progress (
    batch_id         TEXT PRIMARY KEY,
    status           TEXT NOT NULL,
    total_count      BIGINT NOT NULL,
    processed_count  BIGINT NOT NULL,
    failed_count     BIGINT NOT NULL,
    start_time       TIMESTAMPTZ NOT NULL,
    end_time         TIMESTAMPTZ,
    error            TEXT NOT NULL DEFAULT '',
    status_filter    TEXT NOT NULL DEFAULT '',
    time_period      TEXT NOT NULL DEFAULT '',
    event_id         TEXT NOT NULL DEFAULT '',
    expires_at       TIMESTAMPTZ NOT NULL
);

RESET lock_timeout;
RESET statement_timeout;

-- +migrate Up notransaction
SET lock_timeout = '2s';
SET statement_timeout = '30s';

-- Claim path: pending rows whose run_at has arrived, oldest first.
CREATE INDEX IF NOT EXISTS idx_queue_jobs_claim
    ON convoy.queue_jobs (run_at)
    WHERE status = 'pending';

-- Stuck reclaim: processing rows whose claim lease is older than the timeout.
CREATE INDEX IF NOT EXISTS idx_queue_jobs_stuck
    ON convoy.queue_jobs (claimed_at)
    WHERE status = 'processing';

CREATE INDEX IF NOT EXISTS idx_kv_cache_expires
    ON convoy.kv_cache (expires_at)
    WHERE expires_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_batch_retry_progress_expires
    ON convoy.batch_retry_progress (expires_at);

RESET lock_timeout;
RESET statement_timeout;

-- +migrate Down notransaction
SET lock_timeout = '2s';
SET statement_timeout = '30s';

DROP INDEX IF EXISTS convoy.idx_batch_retry_progress_expires;
DROP INDEX IF EXISTS convoy.idx_kv_cache_expires;
DROP INDEX IF EXISTS convoy.idx_queue_jobs_stuck;
DROP INDEX IF EXISTS convoy.idx_queue_jobs_claim;

RESET lock_timeout;
RESET statement_timeout;

-- +migrate Down
SET lock_timeout = '2s';
SET statement_timeout = '30s';

DROP TABLE IF EXISTS convoy.batch_retry_progress;
DROP TABLE IF EXISTS convoy.email_verification_resend_claims;
DROP TABLE IF EXISTS convoy.trial_event_counters;
DROP TABLE IF EXISTS convoy.rate_limits;
DROP TABLE IF EXISTS convoy.kv_cache;
DROP TABLE IF EXISTS convoy.queue_jobs;

RESET lock_timeout;
RESET statement_timeout;
