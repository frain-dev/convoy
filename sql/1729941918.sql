-- +migrate Up
CREATE INDEX IF NOT EXISTS idx_event_deliveries_status ON convoy.event_deliveries (status);
CREATE INDEX IF NOT EXISTS idx_event_deliveries_project_id_endpoint_id_status ON convoy.event_deliveries (project_id, endpoint_id, status);
CREATE INDEX IF NOT EXISTS idx_event_deliveries_project_id_event_id ON convoy.event_deliveries (project_id, event_id);
CREATE INDEX IF NOT EXISTS idx_event_deliveries_project_id_endpoint_id ON convoy.event_deliveries (project_id, endpoint_id);
CREATE INDEX IF NOT EXISTS idx_events_source_id ON convoy.events (source_id);
CREATE INDEX IF NOT EXISTS idx_events_project_id_source_id ON convoy.events (project_id, source_id);

-- +migrate Down
DROP INDEX IF EXISTS idx_event_deliveries_status;
DROP INDEX IF EXISTS idx_event_deliveries_project_id_endpoint_id_status;
DROP INDEX IF EXISTS idx_event_deliveries_project_id_event_id;
DROP INDEX IF EXISTS idx_event_deliveries_project_id_endpoint_id;
DROP INDEX IF EXISTS idx_events_source_id;
DROP INDEX IF EXISTS idx_events_project_id_source_id;
