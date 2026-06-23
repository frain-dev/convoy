-- +migrate Up
ALTER TABLE convoy.event_deliveries ADD COLUMN IF NOT EXISTS target_url TEXT;

-- +migrate Down
ALTER TABLE convoy.event_deliveries DROP COLUMN IF EXISTS target_url;
