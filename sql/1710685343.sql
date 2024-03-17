-- +migrate Up
ALTER TABLE convoy.sources ADD COLUMN IF NOT EXISTS function TEXT;

-- +migrate Down
ALTER TABLE IF EXISTS convoy.sources DROP COLUMN IF EXISTS function;
