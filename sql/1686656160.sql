-- +migrate Up
ALTER TABLE convoy.sources
    ADD COLUMN IF NOT EXISTS idempotency_keys TEXT[],
    ADD COLUMN IF NOT EXISTS idempotency_ttl TEXT;

-- +migrate Up
ALTER TABLE convoy.event_deliveries
    ADD COLUMN IF NOT EXISTS idempotency_key TEXT;

-- +migrate Up
ALTER TABLE convoy.events
    ADD COLUMN IF NOT EXISTS idempotency_key TEXT;

-- +migrate Down
ALTER TABLE convoy.sources
    DROP COLUMN IF EXISTS idempotency_keys,
    DROP COLUMN IF EXISTS idempotency_ttl;

-- +migrate Down
ALTER TABLE convoy.events
    DROP COLUMN IF EXISTS idempotency_key;

-- +migrate Down
ALTER TABLE convoy.event_deliveries
    DROP COLUMN IF EXISTS idempotency_key;
