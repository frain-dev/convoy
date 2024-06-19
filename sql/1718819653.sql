-- +migrate Up
ALTER TABLE convoy.subscriptions ADD COLUMN filter_config_filter_is_flattened BOOLEAN;

-- +migrate Down
ALTER TABLE convoy.subscriptions DROP COLUMN filter_config_filter_is_flattened;
