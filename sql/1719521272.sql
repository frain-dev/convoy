-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';
ALTER TABLE convoy.events ADD COLUMN IF NOT EXISTS acknowledged_at TIMESTAMPTZ DEFAULT NULL;
ALTER TABLE convoy.event_deliveries ADD COLUMN IF NOT EXISTS acknowledged_at TIMESTAMPTZ DEFAULT NULL;

-- +migrate Down
-- squawk-ignore ban-drop-column
ALTER TABLE convoy.events DROP COLUMN IF EXISTS acknowledged_at;
-- squawk-ignore ban-drop-column
ALTER TABLE convoy.event_deliveries DROP COLUMN IF EXISTS acknowledged_at;
