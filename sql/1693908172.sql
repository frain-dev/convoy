-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';
ALTER TABLE convoy.subscriptions ADD COLUMN IF NOT EXISTS function TEXT;

-- +migrate Down
-- squawk-ignore ban-drop-column
ALTER TABLE IF EXISTS convoy.subscriptions DROP COLUMN IF EXISTS function;

