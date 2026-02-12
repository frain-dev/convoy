-- +migrate Up
ALTER TABLE convoy.organisations
    ADD COLUMN IF NOT EXISTS license_data TEXT;

-- +migrate Down
ALTER TABLE convoy.organisations
    DROP COLUMN IF EXISTS license_data;
