-- +migrate Up
ALTER TABLE convoy.event_deliveries ADD COLUMN IF NOT EXISTS delivery_mode convoy.delivery_mode NOT NULL DEFAULT 'at_least_once';
COMMENT ON COLUMN convoy.event_deliveries.delivery_mode IS 'Cached delivery mode from subscription at creation time. Can be either at_least_once or at_most_once';

-- +migrate Down
ALTER TABLE convoy.event_deliveries DROP COLUMN IF EXISTS delivery_mode;
