-- +migrate Up
ALTER TABLE convoy.events ADD COLUMN IF NOT EXISTS acknowledged_at TIMESTAMPTZ DEFAULT NULL;
ALTER TABLE convoy.event_deliveries ADD COLUMN IF NOT EXISTS acknowledged_at TIMESTAMPTZ DEFAULT NULL;

-- +migrate Down
ALTER TABLE convoy.events DROP COLUMN IF EXISTS acknowledged_at;
ALTER TABLE convoy.event_deliveries DROP COLUMN IF EXISTS acknowledged_at;
