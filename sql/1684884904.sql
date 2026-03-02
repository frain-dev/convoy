-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';
-- squawk-ignore changing-column-type
ALTER TABLE convoy.meta_events ALTER COLUMN id TYPE VARCHAR;
-- squawk-ignore changing-column-type
ALTER TABLE convoy.project_configurations ALTER COLUMN id TYPE VARCHAR;
-- squawk-ignore changing-column-type
ALTER TABLE convoy.source_verifiers ALTER COLUMN id TYPE VARCHAR;

-- +migrate Down
-- squawk-ignore changing-column-type
ALTER TABLE convoy.meta_events ALTER COLUMN id TYPE VARCHAR(26);
-- squawk-ignore changing-column-type
ALTER TABLE convoy.project_configurations ALTER COLUMN id TYPE VARCHAR(26);
-- squawk-ignore changing-column-type
ALTER TABLE convoy.source_verifiers ALTER COLUMN id TYPE VARCHAR(26);
