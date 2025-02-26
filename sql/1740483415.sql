-- +migrate Up
CREATE TABLE IF NOT EXISTS convoy.filters (
    id VARCHAR PRIMARY KEY,
    subscription_id VARCHAR NOT NULL,
    event_type VARCHAR NOT NULL,
    headers JSONB NOT NULL DEFAULT '{}'::jsonb,
    body JSONB NOT NULL DEFAULT '{}'::jsonb,
    raw_headers JSONB NOT NULL DEFAULT '{}'::jsonb,
    raw_body JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_subscription
      FOREIGN KEY (subscription_id)
          REFERENCES convoy.subscriptions(id)
          ON DELETE CASCADE
);

CREATE INDEX idx_filters_subscription_id ON convoy.filters(subscription_id);
CREATE INDEX idx_filters_event_type ON convoy.filters(event_type);
CREATE INDEX idx_filters_subscription_event_type ON convoy.filters(subscription_id, event_type);

-- Migrate existing subscription filters to the new filters table. For each subscription event type, create a filter.
INSERT INTO convoy.filters (
    id,
    subscription_id,
    event_type,
    headers,
    body,
    raw_headers,
    raw_body
)
SELECT
    convoy.generate_ulid()::VARCHAR,
    id,
    unnest(filter_config_event_types),
    filter_config_filter_headers,
    filter_config_filter_body,
    filter_config_filter_raw_headers,
    filter_config_filter_raw_body
FROM convoy.subscriptions
WHERE deleted_at IS NULL;

-- +migrate Down
DROP TABLE IF EXISTS convoy.filters;

