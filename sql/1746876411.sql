-- +migrate Up
CREATE TYPE convoy.delivery_mode AS ENUM ('at_least_once', 'at_most_once');

ALTER TABLE convoy.subscriptions ADD COLUMN IF NOT EXISTS delivery_mode convoy.delivery_mode DEFAULT 'at_least_once';
COMMENT ON COLUMN convoy.subscriptions.delivery_mode IS 'Specifies the delivery mode for the subscription. Can be either at_least_once or at_most_once';

-- +migrate Down
ALTER TABLE convoy.subscriptions DROP COLUMN IF EXISTS delivery_mode;
DROP TYPE IF EXISTS convoy.delivery_mode;
