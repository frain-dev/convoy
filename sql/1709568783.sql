-- +migrate Up
CREATE INDEX IF NOT EXISTS idx_project_id_on_not_deleted ON convoy.events(project_id) WHERE deleted_at IS NULL;

-- +migrate Down
DROP INDEX IF EXISTS idx_project_id_on_not_deleted;


