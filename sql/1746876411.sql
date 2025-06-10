-- +migrate Up
CREATE TYPE convoy.delivery_mode AS ENUM ('at_least_once', 'at_most_once');

ALTER TABLE convoy.subscriptions ADD COLUMN IF NOT EXISTS delivery_mode convoy.delivery_mode NOT NULL DEFAULT 'at_least_once';
COMMENT ON COLUMN convoy.subscriptions.delivery_mode IS 'Specifies the delivery mode for the subscription. Can be either at_least_once or at_most_once';
UPDATE convoy.subscriptions SET delivery_mode = 'at_least_once' WHERE delivery_mode IS NULL;

ALTER TABLE convoy.event_deliveries ADD COLUMN IF NOT EXISTS delivery_mode convoy.delivery_mode NOT NULL DEFAULT 'at_least_once';
COMMENT ON COLUMN convoy.event_deliveries.delivery_mode IS 'Cached delivery mode from subscription at creation time. Can be either at_least_once or at_most_once';
UPDATE convoy.event_deliveries SET delivery_mode = 'at_least_once' WHERE delivery_mode IS NULL;

-- +migrate Down
ALTER TABLE convoy.subscriptions DROP COLUMN IF EXISTS delivery_mode;
ALTER TABLE convoy.event_deliveries DROP COLUMN IF EXISTS delivery_mode;
DROP TYPE IF EXISTS convoy.delivery_mode;
