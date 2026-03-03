-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';
-- Add columns to events_search to match events table schema
-- These columns were added to events table but not to events_search, causing queries to fail
ALTER TABLE convoy.events_search ADD COLUMN IF NOT EXISTS acknowledged_at TIMESTAMPTZ DEFAULT NULL;
ALTER TABLE convoy.events_search ADD COLUMN IF NOT EXISTS status TEXT DEFAULT NULL;
ALTER TABLE convoy.events_search ADD COLUMN IF NOT EXISTS metadata TEXT DEFAULT NULL;

-- +migrate Down
ALTER TABLE convoy.events_search DROP COLUMN IF EXISTS acknowledged_at;
ALTER TABLE convoy.events_search DROP COLUMN IF EXISTS status;
ALTER TABLE convoy.events_search DROP COLUMN IF EXISTS metadata;
