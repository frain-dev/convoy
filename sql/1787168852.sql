-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';

-- Claim generation for the postgres queue lease. Heartbeat renews claimed_at
-- and must not change this value: matching only id + status = processing lets
-- a stale worker extend a later Claim after ReclaimStuck, so two handlers run
-- the same job. The next Claim overwrites claim_id; a heartbeat that still
-- carries the old uuid is a no-op.
ALTER TABLE convoy.queue_jobs
    ADD COLUMN IF NOT EXISTS claim_id UUID;

RESET lock_timeout;
RESET statement_timeout;

-- +migrate Down
SET lock_timeout = '2s';
SET statement_timeout = '30s';

ALTER TABLE convoy.queue_jobs
    DROP COLUMN IF EXISTS claim_id;

RESET lock_timeout;
RESET statement_timeout;
