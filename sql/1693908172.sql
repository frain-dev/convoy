-- +migrate Up
ALTER TABLE convoy.subscriptions ADD COLUMN function TEXT;

-- +migrate Down
ALTER TABLE convoy.subscriptions DROP COLUMN IF EXISTS function;

