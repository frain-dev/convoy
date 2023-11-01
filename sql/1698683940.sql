-- +migrate Up
ALTER TABLE convoy.event_deliveries ADD COLUMN IF NOT EXISTS latency TEXT;

-- +migrate Down
ALTER TABLE convoy.event_deliveries DROP COLUMN IF EXISTS latency;

