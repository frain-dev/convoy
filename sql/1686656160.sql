-- squawk-ignore-file ban-drop-column
-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';
ALTER TABLE convoy.sources
    ADD COLUMN IF NOT EXISTS idempotency_keys TEXT[];

-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';
ALTER TABLE convoy.event_deliveries
    ADD COLUMN IF NOT EXISTS idempotency_key TEXT;

-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';
ALTER TABLE convoy.events
    ADD COLUMN IF NOT EXISTS idempotency_key TEXT,
    ADD COLUMN IF NOT EXISTS is_duplicate_event BOOL DEFAULT FALSE;

-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_idempotency_key_key ON convoy.events (idempotency_key);

-- +migrate Down
-- squawk-ignore ban-drop-column
ALTER TABLE convoy.sources
    DROP COLUMN IF EXISTS idempotency_keys;

-- +migrate Down
-- squawk-ignore ban-drop-column
ALTER TABLE convoy.events
    DROP COLUMN IF EXISTS idempotency_key,
    DROP COLUMN if exists is_duplicate_event;

-- +migrate Down
-- squawk-ignore ban-drop-column
ALTER TABLE convoy.event_deliveries
    DROP COLUMN IF EXISTS idempotency_key;

-- +migrate Down
DROP INDEX CONCURRENTLY IF EXISTS convoy.idx_idempotency_key_key;
