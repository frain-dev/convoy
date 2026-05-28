-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';

ALTER TABLE convoy.subscriptions ADD COLUMN IF NOT EXISTS filter_config_filter_query JSONB NOT NULL DEFAULT '{}';
ALTER TABLE convoy.subscriptions ADD COLUMN IF NOT EXISTS filter_config_filter_path JSONB NOT NULL DEFAULT '{}';
ALTER TABLE convoy.subscriptions ADD COLUMN IF NOT EXISTS filter_config_filter_raw_query JSONB NOT NULL DEFAULT '{}';
ALTER TABLE convoy.subscriptions ADD COLUMN IF NOT EXISTS filter_config_filter_raw_path JSONB NOT NULL DEFAULT '{}';

RESET lock_timeout;
RESET statement_timeout;

-- +migrate Down
-- squawk-ignore ban-drop-column
ALTER TABLE convoy.subscriptions DROP COLUMN IF EXISTS filter_config_filter_query;
ALTER TABLE convoy.subscriptions DROP COLUMN IF EXISTS filter_config_filter_path;
ALTER TABLE convoy.subscriptions DROP COLUMN IF EXISTS filter_config_filter_raw_query;
ALTER TABLE convoy.subscriptions DROP COLUMN IF EXISTS filter_config_filter_raw_path;
