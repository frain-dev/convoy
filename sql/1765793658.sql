-- squawk-ignore-file ban-drop-column
-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';
ALTER TABLE convoy.organisations
    ADD COLUMN IF NOT EXISTS disabled_at TIMESTAMPTZ;

-- +migrate Down
-- squawk-ignore ban-drop-column
ALTER TABLE convoy.organisations
    DROP COLUMN IF EXISTS disabled_at;
