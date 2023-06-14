-- +migrate Up
ALTER TABLE convoy.sources ADD COLUMN IF NOT EXISTS idempotency_keys TEXT[];
ALTER TABLE convoy.sources ADD COLUMN IF NOT EXISTS idempotency_ttl TEXT;

-- +migrate Down
ALTER TABLE convoy.sources DROP COLUMN IF EXISTS idempotency_keys;
ALTER TABLE convoy.sources DROP COLUMN IF EXISTS idempotency_ttl;
