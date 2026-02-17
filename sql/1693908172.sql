-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';
ALTER TABLE convoy.subscriptions ADD COLUMN IF NOT EXISTS function TEXT;

-- +migrate Down
ALTER TABLE IF EXISTS convoy.subscriptions DROP COLUMN IF EXISTS function;

