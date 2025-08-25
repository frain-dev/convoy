-- +migrate Up
CREATE INDEX IF NOT EXISTS idx_event_deliveries_project_status_created_deleted
ON convoy.event_deliveries (project_id, status, created_at)
WHERE deleted_at IS NULL;

-- +migrate Down
DROP INDEX IF EXISTS idx_event_deliveries_project_status_created_deleted;
