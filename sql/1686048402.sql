-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';
ALTER TABLE convoy.events ADD COLUMN IF NOT EXISTS url_query_params VARCHAR;
ALTER TABLE convoy.event_deliveries ADD COLUMN IF NOT EXISTS url_query_params VARCHAR;

-- +migrate Down
-- squawk-ignore ban-drop-column
ALTER TABLE convoy.events DROP COLUMN IF EXISTS url_query_params;
-- squawk-ignore ban-drop-column
ALTER TABLE convoy.event_deliveries DROP COLUMN IF EXISTS url_query_params;
