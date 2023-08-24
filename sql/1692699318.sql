-- +migrate Up
ALTER TABLE convoy.project_configurations ADD COLUMN search_policy text;

-- +migrate Up
CREATE TABLE IF NOT EXISTS convoy.jobs(
    id           VARCHAR NOT NULL PRIMARY KEY,
    type         TEXT    NOT NULL,
    status       TEXT    NOT NULL,
    project_id   VARCHAR NOT NULL REFERENCES convoy.projects (id),
    started_at   TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    failed_at    TIMESTAMP WITH TIME ZONE,
    created_at   TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at   TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at   TIMESTAMP WITH TIME ZONE
);

-- +migrate Down
DROP TABLE IF EXISTS convoy.jobs;

-- +migrate Down
ALTER TABLE convoy.project_configurations DROP COLUMN search_policy;
