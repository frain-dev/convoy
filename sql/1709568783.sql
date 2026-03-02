-- +migrate Up notransaction
SET lock_timeout = '2s';
SET statement_timeout = '30s';
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_project_id_on_not_deleted ON convoy.events(project_id) WHERE deleted_at IS NULL;

-- +migrate Down
DROP INDEX CONCURRENTLY IF EXISTS convoy.idx_project_id_on_not_deleted;
