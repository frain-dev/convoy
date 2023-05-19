-- +migrate Up
ALTER TABLE convoy.users ALTER COLUMN id TYPE VARCHAR;
ALTER TABLE convoy.organisations ALTER COLUMN id TYPE VARCHAR;
ALTER TABLE convoy.organisations ALTER COLUMN owner_id TYPE VARCHAR;
ALTER TABLE convoy.projects ALTER COLUMN id TYPE VARCHAR;
ALTER TABLE convoy.projects ALTER COLUMN organisation_id TYPE VARCHAR;
ALTER TABLE convoy.endpoints ALTER COLUMN id TYPE VARCHAR;
ALTER TABLE convoy.endpoints ALTER COLUMN project_id TYPE VARCHAR;
ALTER TABLE convoy.organisation_invites ALTER COLUMN id TYPE VARCHAR;
ALTER TABLE convoy.organisation_invites ALTER COLUMN organisation_id TYPE VARCHAR;

-- +migrate Up
ALTER TABLE convoy.organisation_members ALTER COLUMN id TYPE VARCHAR;
ALTER TABLE convoy.organisation_members ALTER COLUMN user_id TYPE VARCHAR;
ALTER TABLE convoy.organisation_members ALTER COLUMN organisation_id TYPE VARCHAR;

-- +migrate Up
ALTER TABLE convoy.configurations ALTER COLUMN id TYPE VARCHAR;
ALTER TABLE convoy.devices ALTER COLUMN id TYPE VARCHAR;
ALTER TABLE convoy.devices ALTER COLUMN project_id TYPE VARCHAR;
ALTER TABLE convoy.portal_links ALTER COLUMN id TYPE VARCHAR;
ALTER TABLE convoy.portal_links ALTER COLUMN project_id TYPE VARCHAR;

-- +migrate Up
ALTER TABLE convoy.portal_links_endpoints ALTER COLUMN portal_link_id TYPE VARCHAR;
ALTER TABLE convoy.portal_links_endpoints ALTER COLUMN endpoint_id TYPE VARCHAR;


-- +migrate Up
ALTER TABLE convoy.event_deliveries ALTER COLUMN id TYPE VARCHAR;
ALTER TABLE convoy.event_deliveries ALTER COLUMN device_id TYPE VARCHAR;
ALTER TABLE convoy.event_deliveries ALTER COLUMN endpoint_id TYPE VARCHAR;
ALTER TABLE convoy.event_deliveries ALTER COLUMN event_id TYPE VARCHAR;
ALTER TABLE convoy.event_deliveries ALTER COLUMN project_id TYPE VARCHAR;
ALTER TABLE convoy.event_deliveries ALTER COLUMN subscription_id TYPE VARCHAR;

-- +migrate Up
ALTER TABLE convoy.sources ALTER COLUMN id TYPE VARCHAR;
ALTER TABLE convoy.sources ALTER COLUMN project_id TYPE VARCHAR;

-- +migrate Up
ALTER TABLE convoy.subscriptions ALTER COLUMN id TYPE VARCHAR;
ALTER TABLE convoy.subscriptions ALTER COLUMN project_id TYPE VARCHAR;
ALTER TABLE convoy.subscriptions ALTER COLUMN device_id TYPE VARCHAR;
ALTER TABLE convoy.subscriptions ALTER COLUMN endpoint_id TYPE VARCHAR;
ALTER TABLE convoy.subscriptions ALTER COLUMN source_id TYPE VARCHAR;

-- +migrate Up
ALTER TABLE convoy.api_keys ALTER COLUMN id TYPE VARCHAR;
ALTER TABLE convoy.api_keys ALTER COLUMN user_id TYPE VARCHAR;
ALTER TABLE convoy.api_keys ALTER COLUMN role_project TYPE VARCHAR;

-- +migrate Up
ALTER TABLE convoy.events ALTER COLUMN id TYPE VARCHAR;
ALTER TABLE convoy.events_endpoints ALTER COLUMN event_id TYPE VARCHAR;
ALTER TABLE convoy.events_endpoints ALTER COLUMN endpoint_id TYPE VARCHAR;
ALTER TABLE convoy.events ALTER COLUMN project_id TYPE VARCHAR;
ALTER TABLE convoy.events ALTER COLUMN source_id TYPE VARCHAR;

-- +migrate Up
ALTER TABLE convoy.applications ALTER COLUMN id TYPE VARCHAR;
ALTER TABLE convoy.applications ALTER COLUMN project_id TYPE VARCHAR;

-- +migrate Down
ALTER TABLE convoy.users ALTER COLUMN id TYPE CHAR(26);
ALTER TABLE convoy.organisations ALTER COLUMN id TYPE CHAR(26);
ALTER TABLE convoy.organisations ALTER COLUMN owner_id TYPE CHAR(26);
ALTER TABLE convoy.projects ALTER COLUMN id TYPE CHAR(26);
ALTER TABLE convoy.projects ALTER COLUMN organisation_id TYPE CHAR(26);
ALTER TABLE convoy.endpoints ALTER COLUMN id TYPE CHAR(26);
ALTER TABLE convoy.endpoints ALTER COLUMN project_id TYPE CHAR(26);
ALTER TABLE convoy.organisation_invites ALTER COLUMN id TYPE CHAR(26);
ALTER TABLE convoy.organisation_invites ALTER COLUMN organisation_id TYPE CHAR(26);

-- +migrate Down
ALTER TABLE convoy.organisation_members ALTER COLUMN id TYPE CHAR(26);
ALTER TABLE convoy.organisation_members ALTER COLUMN user_id TYPE CHAR(26);
ALTER TABLE convoy.organisation_members ALTER COLUMN organisation_id TYPE CHAR(26);

-- +migrate Down
ALTER TABLE convoy.configurations ALTER COLUMN id TYPE CHAR(26);
ALTER TABLE convoy.devices ALTER COLUMN id TYPE CHAR(26);
ALTER TABLE convoy.devices ALTER COLUMN project_id TYPE CHAR(26);
ALTER TABLE convoy.portal_links ALTER COLUMN id TYPE CHAR(26);
ALTER TABLE convoy.portal_links ALTER COLUMN project_id TYPE CHAR(26);

-- +migrate Down
ALTER TABLE convoy.portal_links_endpoints ALTER COLUMN portal_link_id TYPE CHAR(26);
ALTER TABLE convoy.portal_links_endpoints ALTER COLUMN endpoint_id TYPE CHAR(26);


-- +migrate Down
ALTER TABLE convoy.event_deliveries ALTER COLUMN id TYPE CHAR(26);
ALTER TABLE convoy.event_deliveries ALTER COLUMN device_id TYPE CHAR(26);
ALTER TABLE convoy.event_deliveries ALTER COLUMN endpoint_id TYPE CHAR(26);
ALTER TABLE convoy.event_deliveries ALTER COLUMN event_id TYPE CHAR(26);
ALTER TABLE convoy.event_deliveries ALTER COLUMN project_id TYPE CHAR(26);
ALTER TABLE convoy.event_deliveries ALTER COLUMN subscription_id TYPE CHAR(26);

-- +migrate Down
ALTER TABLE convoy.sources ALTER COLUMN id TYPE CHAR(26);
ALTER TABLE convoy.sources ALTER COLUMN project_id TYPE CHAR(26);

-- +migrate Down
ALTER TABLE convoy.subscriptions ALTER COLUMN id TYPE CHAR(26);
ALTER TABLE convoy.subscriptions ALTER COLUMN project_id TYPE CHAR(26);
ALTER TABLE convoy.subscriptions ALTER COLUMN device_id TYPE CHAR(26);
ALTER TABLE convoy.subscriptions ALTER COLUMN endpoint_id TYPE CHAR(26);
ALTER TABLE convoy.subscriptions ALTER COLUMN source_id TYPE CHAR(26);

-- +migrate Down
ALTER TABLE convoy.api_keys ALTER COLUMN id TYPE CHAR(26);
ALTER TABLE convoy.api_keys ALTER COLUMN user_id TYPE CHAR(26);
ALTER TABLE convoy.api_keys ALTER COLUMN role_project TYPE CHAR(26);

-- +migrate Down
ALTER TABLE convoy.events ALTER COLUMN id TYPE CHAR(26);
ALTER TABLE convoy.events_endpoints ALTER COLUMN event_id TYPE CHAR(26);
ALTER TABLE convoy.events_endpoints ALTER COLUMN endpoint_id TYPE CHAR(26);
ALTER TABLE convoy.events ALTER COLUMN project_id TYPE CHAR(26);
ALTER TABLE convoy.events ALTER COLUMN source_id TYPE CHAR(26);

-- +migrate Down
ALTER TABLE convoy.applications ALTER COLUMN id TYPE CHAR(26);
ALTER TABLE convoy.applications ALTER COLUMN project_id TYPE CHAR(26);

