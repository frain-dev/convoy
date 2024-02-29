-- +migrate Up
CREATE TABLE IF NOT EXISTS convoy.event_catalogues (
    id CHAR(26) PRIMARY KEY,

    project_id TEXT NOT NULL,
    type TEXT NOT NULL,
    events jsonb,
    open_api_spec bytea,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ
);

-- +migrate Down
DROP TABLE IF EXISTS convoy.event_catalogues;
