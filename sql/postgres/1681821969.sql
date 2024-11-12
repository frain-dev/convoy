-- +migrate Up
CREATE TABLE IF NOT EXISTS convoy.meta_events (
    id CHAR(26) PRIMARY KEY,

    event_type TEXT NOT NULL,
    project_id CHAR(26) NOT NULL REFERENCES convoy.projects (id),
    metadata JSONB NOT NULL,
    attempt JSONB,
    status TEXT NOT NULL,

    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ
);

--+ migrate Up
-- convoy.project_configurations
ALTER TABLE convoy.project_configurations ADD meta_events_enabled BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE convoy.project_configurations ADD meta_events_type TEXT;
ALTER TABLE convoy.project_configurations ADD meta_events_event_type TEXT; 
ALTER TABLE convoy.project_configurations ADD meta_events_url TEXT;
ALTER TABLE convoy.project_configurations add meta_events_secret TEXT;
ALTER TABLE convoy.project_configurations ADD meta_events_pub_sub JSONB;

-- +migrate Down
DROP TABLE IF EXISTS convoy.meta_events;
ALTER TABLE convoy.project_configurations DROP COLUMN meta_events_enabled;
ALTER TABLE convoy.project_configurations DROP COLUMN meta_events_type;
ALTER TABLE convoy.project_configurations DROP COLUMN meta_events_event_type;
ALTER TABLE convoy.project_configurations DROP COLUMN meta_events_url;
ALTER TABLE convoy.project_configurations DROP COLUMN meta_events_secret;
ALTER TABLE convoy.project_configurations DROP COLUMN meta_events_pub_sub;
