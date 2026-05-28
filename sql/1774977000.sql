-- +migrate Up
ALTER TABLE convoy.sources
    ADD COLUMN IF NOT EXISTS event_type_location TEXT NOT NULL DEFAULT '';

-- +migrate Down
ALTER TABLE convoy.sources
    DROP COLUMN IF EXISTS event_type_location;
