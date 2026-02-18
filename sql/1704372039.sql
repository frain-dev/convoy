-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';
ALTER TABLE convoy.event_deliveries ADD COLUMN IF NOT EXISTS event_type TEXT;

-- +migrate Up notransaction
SET lock_timeout = '2s';
SET statement_timeout = '30s';
CREATE INDEX CONCURRENTLY IF NOT EXISTS event_deliveries_event_type_1 ON convoy.event_deliveries(event_type);

-- +migrate Down notransaction
DROP INDEX CONCURRENTLY IF EXISTS convoy.event_deliveries_event_type_1;

-- +migrate Down
-- squawk-ignore ban-drop-column
ALTER TABLE convoy.event_deliveries DROP COLUMN IF EXISTS event_type;
