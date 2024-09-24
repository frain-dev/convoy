-- +migrate Up
ALTER TABLE convoy.event_deliveries ADD COLUMN IF NOT EXISTS latency_seconds numeric DEFAULT NULL;

-- +migrate Down
ALTER TABLE convoy.event_deliveries DROP COLUMN IF EXISTS latency_seconds;
