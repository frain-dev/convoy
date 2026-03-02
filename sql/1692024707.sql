-- +migrate Up notransaction
SET lock_timeout = '2s';
SET statement_timeout = '30s';
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS endpoints_title_project_id_pk_idx
    ON convoy.endpoints (title, project_id, deleted_at);

-- +migrate Up
ALTER TABLE convoy.endpoints
    ADD CONSTRAINT endpoints_title_project_id_pk
        UNIQUE USING INDEX endpoints_title_project_id_pk_idx;

-- +migrate Down
ALTER TABLE convoy.endpoints
    DROP CONSTRAINT IF EXISTS endpoints_title_project_id_pk;

-- +migrate Down
DROP INDEX CONCURRENTLY IF EXISTS convoy.endpoints_title_project_id_pk_idx;
