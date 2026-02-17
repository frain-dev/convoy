-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';
ALTER TABLE convoy.event_deliveries ADD COLUMN IF NOT EXISTS latency TEXT;

-- +migrate Down
ALTER TABLE convoy.event_deliveries DROP COLUMN IF EXISTS latency;

