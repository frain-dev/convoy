-- +migrate Up
CREATE INDEX IF NOT EXISTS idx_endpoints_project_id_key ON convoy.endpoints (project_id);
CREATE INDEX IF NOT EXISTS idx_endpoints_owner_id_key ON convoy.endpoints (owner_id);

-- +migrate Up
CREATE INDEX IF NOT EXISTS idx_organisation_members_organisation_id_key ON convoy.organisation_members (organisation_id);
CREATE INDEX IF NOT EXISTS idx_organisation_members_user_id_key ON convoy.organisation_members (user_id);

-- +migrate Up
CREATE INDEX IF NOT EXISTS idx_events_project_id_key ON convoy.events (project_id);

-- +migrate Up
CREATE INDEX IF NOT EXISTS idx_events_endpoints_endpoint_id_key ON convoy.events_endpoints (endpoint_id);

-- +migrate Up
CREATE INDEX IF NOT EXISTS idx_event_deliveries_project_id_key ON convoy.event_deliveries (project_id);
CREATE INDEX IF NOT EXISTS idx_event_deliveries_status_key ON convoy.event_deliveries (status);


-- +migrate Down
DROP INDEX IF EXISTS convoy.idx_endpoints_project_id_key, convoy.idx_endpoints_owner_id_key;
DROP INDEX IF EXISTS convoy.idx_organisation_members_organisation_id_key, convoy.idx_organisation_members_user_id_key;
DROP INDEX IF EXISTS convoy.idx_events_project_id_key;
DROP INDEX IF EXISTS convoy.idx_events_endpoints_endpoint_id_key;
DROP INDEX IF EXISTS convoy.idx_event_deliveries_project_id_key, convoy.idx_event_deliveries_status_key;