-- +migrate Up
ALTER TABLE convoy.events ADD COLUMN IF NOT EXISTS url_query_params VARCHAR;
ALTER TABLE convoy.event_deliveries ADD COLUMN IF NOT EXISTS url_query_params VARCHAR;

-- +migrate Down
ALTER TABLE convoy.events DROP COLUMN IF EXISTS url_query_params;
ALTER TABLE convoy.event_deliveries DROP COLUMN IF EXISTS url_query_params;
