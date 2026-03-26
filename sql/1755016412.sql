-- +migrate Up notransaction
SET lock_timeout = '2s';
SET statement_timeout = '30s';
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_event_deliveries_project_status_created_deleted
ON convoy.event_deliveries (project_id, status, created_at)
WHERE deleted_at IS NULL;

-- +migrate Down
DROP INDEX CONCURRENTLY IF EXISTS idx_event_deliveries_project_status_created_deleted;
