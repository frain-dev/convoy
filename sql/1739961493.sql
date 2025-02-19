-- +migrate Up
ALTER TABLE convoy.projects DROP CONSTRAINT IF EXISTS projects_project_configuration_id_fkey;
ALTER TABLE convoy.meta_events DROP CONSTRAINT IF EXISTS meta_events_project_id_fkey;
ALTER TABLE convoy.sources DROP CONSTRAINT IF EXISTS sources_source_verifier_id_fkey;

-- +migrate Up
ALTER TABLE convoy.projects
    ALTER COLUMN project_configuration_id TYPE VARCHAR USING project_configuration_id::VARCHAR;

ALTER TABLE convoy.meta_events
    ALTER COLUMN project_id TYPE VARCHAR USING project_id::VARCHAR;

ALTER TABLE convoy.sources
    ALTER COLUMN source_verifier_id TYPE VARCHAR USING source_verifier_id::VARCHAR;

-- +migrate Up
ALTER TABLE convoy.projects
    ADD CONSTRAINT projects_project_configuration_id_fkey
        FOREIGN KEY (project_configuration_id)
            REFERENCES convoy.project_configurations(id)
            ON DELETE CASCADE;

ALTER TABLE convoy.meta_events
    ADD CONSTRAINT meta_events_project_id_fkey
        FOREIGN KEY (project_id)
            REFERENCES convoy.projects(id)
            ON DELETE CASCADE;

ALTER TABLE convoy.sources
    ADD CONSTRAINT sources_source_verifier_id_fkey
        FOREIGN KEY (source_verifier_id)
            REFERENCES convoy.source_verifiers(id)
            ON DELETE SET NULL;

-- +migrate Up
DROP INDEX IF EXISTS convoy.idx_sources_source_verifier_id;
CREATE INDEX idx_sources_source_verifier_id ON convoy.sources (source_verifier_id);

-- +migrate Up
REINDEX TABLE convoy.projects;
REINDEX TABLE convoy.meta_events;
REINDEX TABLE convoy.sources;

-- +migrate Down
ALTER TABLE convoy.projects DROP CONSTRAINT IF EXISTS projects_project_configuration_id_fkey;
ALTER TABLE convoy.meta_events DROP CONSTRAINT IF EXISTS meta_events_project_id_fkey;
ALTER TABLE convoy.sources DROP CONSTRAINT IF EXISTS sources_source_verifier_id_fkey;

-- +migrate Down
ALTER TABLE convoy.projects
    ALTER COLUMN project_configuration_id TYPE CHAR(26) USING project_configuration_id::CHAR(26);
ALTER TABLE convoy.meta_events
    ALTER COLUMN project_id TYPE CHAR(26) USING project_id::CHAR(26);
ALTER TABLE convoy.sources
    ALTER COLUMN source_verifier_id TYPE CHAR(26) USING source_verifier_id::CHAR(26);

-- +migrate Down
ALTER TABLE convoy.projects
    ADD CONSTRAINT projects_project_configuration_id_fkey
        FOREIGN KEY (project_configuration_id)
            REFERENCES convoy.project_configurations(id)
            ON DELETE CASCADE;

ALTER TABLE convoy.meta_events
    ADD CONSTRAINT meta_events_project_id_fkey
        FOREIGN KEY (project_id)
            REFERENCES convoy.projects(id)
            ON DELETE CASCADE;

ALTER TABLE convoy.sources
    ADD CONSTRAINT sources_source_verifier_id_fkey
        FOREIGN KEY (source_verifier_id)
            REFERENCES convoy.source_verifiers(id)
            ON DELETE SET NULL;

-- +migrate Down
DROP INDEX IF EXISTS convoy.idx_sources_source_verifier_id;
CREATE INDEX idx_sources_source_verifier_id ON convoy.sources (source_verifier_id);

-- +migrate Down
REINDEX TABLE convoy.projects;
REINDEX TABLE convoy.meta_events;
REINDEX TABLE convoy.sources;
