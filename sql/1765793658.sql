-- +migrate Up
ALTER TABLE convoy.organisations
    ADD COLUMN IF NOT EXISTS disabled_at TIMESTAMPTZ;

-- +migrate Down
ALTER TABLE convoy.organisations
    DROP COLUMN IF EXISTS disabled_at;
