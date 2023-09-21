-- +migrate Up
ALTER TABLE convoy.sources
    ADD COLUMN IF NOT EXISTS idempotency_keys TEXT[];

-- +migrate Up
ALTER TABLE convoy.event_deliveries
    ADD COLUMN IF NOT EXISTS idempotency_key TEXT;

-- +migrate Up
ALTER TABLE convoy.events
    ADD COLUMN IF NOT EXISTS idempotency_key TEXT,
    ADD COLUMN IF NOT EXISTS is_duplicate_event BOOL DEFAULT FALSE;

-- +migrate Up
CREATE INDEX IF NOT EXISTS idx_idempotency_key_key ON convoy.events (idempotency_key);

-- +migrate Down
ALTER TABLE convoy.sources
    DROP COLUMN IF EXISTS idempotency_keys;

-- +migrate Down
ALTER TABLE convoy.events
    DROP COLUMN IF EXISTS idempotency_key,
    DROP COLUMN if exists is_duplicate_event;

-- +migrate Down
ALTER TABLE convoy.event_deliveries
    DROP COLUMN IF EXISTS idempotency_key;

-- +migrate Down
DROP INDEX IF EXISTS convoy.idx_idempotency_key_key;
