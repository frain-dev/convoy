-- +migrate Up
-- convoy.endpoints
CREATE INDEX IF NOT EXISTS idx_endpoints_project_id_key ON convoy.endpoints (project_id);
CREATE INDEX IF NOT EXISTS idx_endpoints_owner_id_key ON convoy.endpoints (owner_id);

-- +migrate Up
-- convoy.organisation_members
CREATE INDEX IF NOT EXISTS idx_organisation_members_organisation_id_key ON convoy.organisation_members (organisation_id);
CREATE INDEX IF NOT EXISTS idx_organisation_members_user_id_key ON convoy.organisation_members (user_id);

-- +migrate Up
-- convoy.events
CREATE INDEX IF NOT EXISTS idx_events_project_id_key ON convoy.events (project_id);
CREATE INDEX IF NOT EXISTS idx_events_source_id_key ON convoy.events (source_id);
CREATE INDEX IF NOT EXISTS idx_events_created_at_key ON convoy.events (created_at);
CREATE INDEX IF NOT EXISTS idx_events_deleted_at_key ON convoy.events (deleted_at);


-- +migrate Up
-- convoy.events_endpoints
CREATE INDEX IF NOT EXISTS idx_events_endpoints_endpoint_id_key ON convoy.events_endpoints (endpoint_id);
CREATE INDEX IF NOT EXISTS idx_events_endpoints_event_id_key ON convoy.events_endpoints (event_id);

-- +migrate Up
-- convoy.event_deliveries
CREATE INDEX IF NOT EXISTS idx_event_deliveries_project_id_key ON convoy.event_deliveries (project_id);
CREATE INDEX IF NOT EXISTS idx_event_deliveries_status_key ON convoy.event_deliveries (status);
CREATE INDEX IF NOT EXISTS idx_event_deliveries_event_id_key ON convoy.event_deliveries(event_id);

--+ migrate Up
-- convoy.organisations
CREATE UNIQUE INDEX IF NOT EXISTS organisations_custom_domain ON convoy.organisations(custom_domain, assigned_domain) WHERE deleted_at IS NULL;

--+ migrate Up
-- convoy.organisation_invites
CREATE UNIQUE INDEX IF NOT EXISTS organisation_invites_invitee_email ON convoy.organisation_invites(organisation_id, invitee_email) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_organisation_invites_token_key ON convoy.organisation_invites (token);

--+ migrate Up
-- convoy.api_keys
CREATE INDEX IF NOT EXISTS idx_api_keys_mask_id ON convoy.api_keys (mask_id);


-- +migrate Down
DROP INDEX IF EXISTS convoy.idx_endpoints_project_id_key, convoy.idx_endpoints_owner_id_key;
DROP INDEX IF EXISTS convoy.idx_organisation_members_organisation_id_key, convoy.idx_organisation_members_user_id_key;
DROP INDEX IF EXISTS convoy.idx_events_project_id_key;
DROP INDEX IF EXISTS convoy.idx_events_endpoints_endpoint_id_key;
DROP INDEX IF EXISTS convoy.idx_event_deliveries_project_id_key, convoy.idx_event_deliveries_status_key, convoy.idx_event_deliveries_event_id_key;
DROP INDEX IF EXISTS convoy.organisations_custom_domain;
DROP INDEX IF EXISTS convoy.organisation_invites_invitee_email, convoy.idx_organisation_invites_token_key;
DROP INDEX IF EXISTS convoy.idx_api_keys_mask_id;