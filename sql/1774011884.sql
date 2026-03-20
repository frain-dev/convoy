-- +migrate Up
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_endpoints_project_id_url_unique
    ON convoy.endpoints (project_id, url) WHERE deleted_at IS NULL;

-- +migrate Down
DROP INDEX CONCURRENTLY IF EXISTS convoy.idx_endpoints_project_id_url_unique;
