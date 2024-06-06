-- +migrate Up
DROP INDEX IF EXISTS convoy.idx_fetch_subscriptions_for_broadcast;
DROP INDEX IF EXISTS convoy.idx_subscriptions_project_id_key;

CREATE INDEX IF NOT EXISTS idx_subscriptions_id_deleted_at ON convoy.subscriptions (id,deleted_at) WHERE deleted_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_subscriptions_updated_at ON convoy.subscriptions USING BRIN(updated_at) WHERE deleted_at IS NULL;


-- +migrate Down
CREATE INDEX IF NOT EXISTS idx_subscriptions_project_id_key ON convoy.subscriptions (id,project_id) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT
    EXISTS idx_fetch_subscriptions_for_broadcast
    ON convoy.subscriptions (
                             project_id,
                             id
        )
    INCLUDE (
        type,
        endpoint_id,
        function,
        filter_config_filter_headers,
        filter_config_event_types,
        filter_config_filter_body
        )
    WHERE deleted_at IS NULL;

DROP INDEX IF EXISTS convoy.idx_subscriptions_project_id_key;
DROP INDEX IF EXISTS convoy.idx_subscriptions_updated_at;
