-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';
ALTER TABLE convoy.organisations
    ADD COLUMN IF NOT EXISTS license_data TEXT;

-- +migrate Down
ALTER TABLE convoy.organisations
    DROP COLUMN IF EXISTS license_data;
