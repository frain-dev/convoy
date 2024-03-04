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

CREATE UNIQUE INDEX IF NOT EXISTS event_catalogues_project_id ON convoy.event_catalogues(project_id) WHERE deleted_at IS NULL;

-- +migrate Down
DROP TABLE IF EXISTS convoy.event_catalogues;
DROP INDEX IF EXISTS convoy.event_catalogues_project_id;
