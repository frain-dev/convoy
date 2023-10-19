-- +migrate Up
ALTER TABLE convoy.subscriptions ADD COLUMN IF NOT EXISTS function TEXT;

-- +migrate Down
ALTER TABLE IF EXISTS convoy.subscriptions DROP COLUMN IF EXISTS function;

