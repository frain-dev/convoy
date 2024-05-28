-- +migrate Up
ALTER TABLE convoy.event_deliveries ADD COLUMN IF NOT EXISTS event_type TEXT;
CREATE INDEX IF NOT EXISTS event_deliveries_event_type_1 ON convoy.event_deliveries(event_type);

-- +migrate Down
ALTER TABLE convoy.event_deliveries DROP COLUMN IF EXISTS event_type;
DROP INDEX IF EXISTS convoy.event_deliveries_event_type_1;
