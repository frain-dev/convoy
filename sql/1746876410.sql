-- +migrate Up
CREATE TYPE delivery_mode AS ENUM ('at_least_once', 'at_most_once');
ALTER TABLE convoy.subscriptions
  ADD COLUMN IF NOT EXISTS delivery_mode delivery_mode NOT NULL DEFAULT 'at_least_once';

-- Add comment to explain the column
COMMENT ON COLUMN convoy.subscriptions.delivery_mode IS 'Specifies the delivery mode for the subscription. Can be either at_least_once or at_most_once';

-- +migrate Down
ALTER TABLE convoy.subscriptions DROP COLUMN IF EXISTS delivery_mode; 