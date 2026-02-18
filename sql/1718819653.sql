-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';
ALTER TABLE convoy.subscriptions ADD COLUMN filter_config_filter_is_flattened BOOLEAN DEFAULT false;

-- +migrate Down
-- squawk-ignore ban-drop-column
ALTER TABLE convoy.subscriptions DROP COLUMN filter_config_filter_is_flattened;
