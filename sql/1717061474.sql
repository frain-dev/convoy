-- +migrate Up
CREATE INDEX IF NOT EXISTS idx_subscriptions_project_id_endpoint_id_key ON convoy.subscriptions (project_id,endpoint_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_subscriptions_project_id_only_key ON convoy.subscriptions (project_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_subscriptions_name_key ON convoy.subscriptions (name) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_subscriptions_project_id_filter_config_event_types_key ON convoy.subscriptions (project_id,filter_config_event_types) WHERE deleted_at IS NULL;

-- +migrate Down
DROP INDEX IF EXISTS convoy.idx_subscriptions_name_key;
DROP INDEX IF EXISTS convoy.idx_subscriptions_project_id_only_key;
DROP INDEX IF EXISTS convoy.idx_subscriptions_project_id_endpoint_id_key;
DROP INDEX IF EXISTS convoy.idx_subscriptions_project_id_filter_config_event_types_key;
