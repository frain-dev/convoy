-- +migrate Up
CREATE TABLE IF NOT EXISTS convoy.backup_jobs (
    id            VARCHAR PRIMARY KEY DEFAULT convoy.generate_ulid(),
    hour_start    TIMESTAMPTZ NOT NULL,
    hour_end      TIMESTAMPTZ NOT NULL,
    status        VARCHAR NOT NULL DEFAULT 'pending',
    worker_id     VARCHAR,
    claimed_at    TIMESTAMPTZ,
    completed_at  TIMESTAMPTZ,
    error         TEXT,
    record_counts JSONB,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +migrate Up notransaction
CREATE INDEX idx_backup_jobs_pending ON convoy.backup_jobs(status, created_at)
    WHERE status IN ('pending', 'claimed');

-- +migrate Down
DROP TABLE IF EXISTS convoy.backup_jobs;
