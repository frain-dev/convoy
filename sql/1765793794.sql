-- +migrate Up notransaction
SET lock_timeout = '2s';
SET statement_timeout = '30s';
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_events_project_created_deleted 
ON convoy.events(project_id, created_at, deleted_at)
WHERE deleted_at IS NULL;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_event_deliveries_project_status_created_deleted 
ON convoy.event_deliveries(project_id, status, created_at, deleted_at)
WHERE deleted_at IS NULL AND status = 'Success';

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_events_id_deleted 
ON convoy.events(id, deleted_at)
WHERE deleted_at IS NULL;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_projects_organisation_deleted 
ON convoy.projects(organisation_id, deleted_at)
WHERE deleted_at IS NULL;

-- +migrate Down
DROP INDEX CONCURRENTLY IF EXISTS convoy.idx_projects_organisation_deleted;
DROP INDEX CONCURRENTLY IF EXISTS convoy.idx_events_id_deleted;
DROP INDEX CONCURRENTLY IF EXISTS convoy.idx_event_deliveries_project_status_created_deleted;
DROP INDEX CONCURRENTLY IF EXISTS convoy.idx_events_project_created_deleted;
