-- +migrate Up
ALTER TABLE convoy.event_deliveries ADD COLUMN IF NOT EXISTS endpoint_url TEXT;

-- +migrate Down
ALTER TABLE convoy.event_deliveries DROP COLUMN IF EXISTS endpoint_url;
