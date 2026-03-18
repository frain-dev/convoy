-- squawk-ignore-file ban-drop-column
-- +migrate Up
SET lock_timeout = '2min';
SET statement_timeout = '10min';
ALTER TABLE convoy.organisations
    ADD COLUMN IF NOT EXISTS license_data TEXT;

RESET lock_timeout;
RESET statement_timeout;

-- +migrate Down
-- squawk-ignore ban-drop-column
ALTER TABLE convoy.organisations
    DROP COLUMN IF EXISTS license_data;
