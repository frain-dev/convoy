-- +migrate Up
ALTER TABLE convoy.projects ADD COLUMN IF NOT EXISTS disabled_by_license BOOLEAN DEFAULT FALSE;
-- +migrate Down
ALTER TABLE convoy.projects DROP COLUMN IF EXISTS disabled_by_license;
