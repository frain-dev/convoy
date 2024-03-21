-- +migrate Up
ALTER TABLE IF EXISTS convoy.sources DROP COLUMN IF EXISTS function;
ALTER TABLE convoy.sources ADD COLUMN IF NOT EXISTS body_function TEXT;
ALTER TABLE convoy.sources ADD COLUMN IF NOT EXISTS header_function TEXT;

-- +migrate Down
ALTER TABLE IF EXISTS convoy.sources DROP COLUMN IF EXISTS body_function;
ALTER TABLE IF EXISTS convoy.sources DROP COLUMN IF EXISTS header_function;
