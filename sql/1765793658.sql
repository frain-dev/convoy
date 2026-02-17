-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';
ALTER TABLE convoy.organisations
    ADD COLUMN IF NOT EXISTS disabled_at TIMESTAMPTZ;

-- +migrate Down
ALTER TABLE convoy.organisations
    DROP COLUMN IF EXISTS disabled_at;
