-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';
ALTER TABLE convoy.meta_events ALTER COLUMN id TYPE VARCHAR;
ALTER TABLE convoy.project_configurations ALTER COLUMN id TYPE VARCHAR;
ALTER TABLE convoy.source_verifiers ALTER COLUMN id TYPE VARCHAR;

-- +migrate Down
ALTER TABLE convoy.meta_events ALTER COLUMN id TYPE VARCHAR(26);
ALTER TABLE convoy.project_configurations ALTER COLUMN id TYPE VARCHAR(26);
ALTER TABLE convoy.source_verifiers ALTER COLUMN id TYPE VARCHAR(26);
