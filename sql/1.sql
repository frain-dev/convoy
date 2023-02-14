-- +migrate Up
CREATE SCHEMA IF NOT EXISTS convoy;

CREATE TABLE IF NOT EXISTS convoy.users (
    id CHAR(26) PRIMARY KEY,

    first_name TEXT NOT NULL,
    last_name TEXT NOT NULL,
    email TEXT NOT NULL,
    password TEXT NOT NULL,
    email_verified BOOL NOT NULL,
    reset_password_token TEXT,
    email_verification_token TEXT,

    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ,
    reset_password_expires_at TIMESTAMPTZ,
    email_verification_expires_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS convoy.organisations (
	id CHAR(26) PRIMARY KEY,

	name TEXT NOT NULL,
	owner_id CHAR(26) NOT NULL REFERENCES convoy.users (id),
	custom_domain TEXT,
	assigned_domain TEXT,

	created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
	deleted_at TIMESTAMPTZ NULL
);

CREATE TABLE IF NOT EXISTS convoy.organisation_members (
	id CHAR(26) PRIMARY KEY,

	role_type TEXT NOT NULL,
	role_project TEXT NOT NULL,
	role_endpoint TEXT NOT NULL,
	user_id CHAR(26) NOT NULL REFERENCES convoy.users (id),
	organisation_id CHAR(26) NOT NULL REFERENCES convoy.organisations (id),

	created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
	deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS convoy.project_configurations (
	id CHAR(26) PRIMARY KEY,

	retention_policy TEXT NOT NULL,
	max_payload_read_size INTEGER NOT NULL,

	replay_attacks_prevention_enabled BOOLEAN NOT NULL,
	retention_policy_enabled BOOLEAN NOT NULL,

	ratelimit_count INTEGER NOT NULL,
	ratelimit_duration INTEGER NOT NULL,

	strategy_type TEXT NOT NULL,
	strategy_duration INTEGER NOT NULL,
	strategy_retry_count INTEGER NOT NULL,

	signature_header TEXT NOT NULL,
	signature_versions JSONB NOT NULL,

	created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
	deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS convoy.projects (
	id CHAR(26) PRIMARY KEY,

	name TEXT NOT NULL,
	type TEXT NOT NULL,
	logo_url TEXT,
	retained_events INTEGER DEFAULT 0,

	organisation_id CHAR(26) NOT NULL REFERENCES convoy.organisations (id),
	project_configuration_id CHAR(26) NOT NULL REFERENCES convoy.project_configurations (id),

	created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
	deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS convoy.endpoints (
	id CHAR(26) PRIMARY KEY,

	title TEXT NOT NULL,
	status TEXT NOT NULL,
	owner_id TEXT,
	target_url TEXT NOT NULL,
	description TEXT,
	http_timeout TEXT NOT NULL,
	rate_limit INTEGER NOT NULL,
	rate_limit_duration TEXT NOT NULL,
	advanced_signatures BOOLEAN NOT NULL,

	slack_webhook_url TEXT,
	support_email TEXT,
	app_id TEXT,

	project_id CHAR(26) NOT NULL REFERENCES convoy.projects (id),

	authentication_type TEXT,
	authentication_type_api_key_header_name TEXT,
	authentication_type_api_key_header_value TEXT,

	created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
	deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS convoy.endpoint_secrets (
	id CHAR(26) PRIMARY KEY,

	value TEXT NOT NULL,
	endpoint_id CHAR(26) NOT NULL REFERENCES convoy.endpoints (id),

	created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
	expires_at TIMESTAMPTZ,
	deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS convoy.applications (
	id CHAR(26) PRIMARY KEY,

	project_id CHAR(26) NOT NULL REFERENCES convoy.projects (id),
	title TEXT NOT NULL,
	support_email TEXT,
	slack_webhook_url TEXT,

	created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
	deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS convoy.organisation_invites (
	id CHAR(26) PRIMARY KEY,

	organisation_id CHAR(26) NOT NULL REFERENCES convoy.organisations (id),
	invitee_email TEXT NOT NULL,
	token TEXT NOT NULL,
	role_type TEXT NOT NULL,
	role_project TEXT NOT NULL,
	role_endpoint TEXT NOT NULL,
	status TEXT NOT NULL,

	created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
	expires_at TIMESTAMPTZ NOT NULL,
	deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS convoy.portal_links (
	id CHAR(26) PRIMARY KEY,

	project_id CHAR(26) NOT NULL REFERENCES convoy.projects (id),

	name TEXT NOT NULL,
	token TEXT NOT NULL,
	endpoints TEXT[] NOT NULL,

	created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
	deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS convoy.subscription_filters (
	id CHAR(26) PRIMARY KEY,
	invitee_email JSONB NOT NULL,
	deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS convoy.devices (
	id CHAR(26) PRIMARY KEY,

	project_id CHAR(26) NOT NULL REFERENCES convoy.projects (id),
	endpoint_id CHAR(26) NOT NULL REFERENCES convoy.endpoints (id),

	host_name TEXT NOT NULL,
	status TEXT NOT NULL,

	created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
	last_seen_at TIMESTAMPTZ NOT NULL,
	deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS convoy.configurations (
	id TEXT PRIMARY KEY,

	is_analytics_enabled TEXT NOT NULL,
	is_signup_enabled BOOLEAN NOT NULL,
	storage_policy_type TEXT NOT NULL,

	-- on-prem
	on_prem_path TEXT,

	-- s3 storage
	s3_bucket TEXT,
	s3_access_key TEXT,
	s3_secret_key TEXT,
	s3_region TEXT,
	s3_session_token TEXT,
	s3_endpoint TEXT,

	created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
	deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS convoy.sources (
	id CHAR(26) PRIMARY KEY,

	name TEXT NOT NULL,
	type TEXT NOT NULL,
	mask_id TEXT NOT NULL,
	provider TEXT NOT NULL,
	is_disabled BOOLEAN NOT NULL,
	forward_headers TEXT[],

	project_id CHAR(26) NOT NULL REFERENCES convoy.projects (id),

	created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
	deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS convoy.source_verifiers (
	id CHAR(26) PRIMARY KEY,

	type TEXT NOT NULL,

	basic_username TEXT,
	basic_password TEXT,

	api_key_header_name TEXT,
	api_key_header_value TEXT,

	hmac_hash TEXT,
    hmac_header TEXT,
	hmac_secret TEXT,
	hmac_encoding TEXT,

	twitter_crc_verified_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,

	source_id CHAR(26) NOT NULL REFERENCES convoy.sources (id)
);

CREATE TABLE IF NOT EXISTS convoy.subscriptions (
	id CHAR(26) PRIMARY KEY,

	name TEXT NOT NULL,
	type TEXT NOT NULL,

	project_id CHAR(26) NOT NULL REFERENCES convoy.projects (id),
	endpoint_id CHAR(26) NOT NULL REFERENCES convoy.endpoints (id),
	device_id CHAR(26) NOT NULL REFERENCES convoy.devices (id),
	source_id CHAR(26) NOT NULL REFERENCES convoy.sources (id),

	alert_config_count INTEGER NOT NULL,
	alert_config_threshold TEXT NOT NULL,

	retry_config_type TEXT NOT NULL,
	retry_config_duration INTEGER NOT NULL,
	retry_config_retry_count INTEGER NOT NULL,

	filter_config_event_types TEXT[] NOT NULL,
	filter_config_filter_headers JSONB NOT NULL,
	filter_config_filter_body JSONB NOT NULL,

	rate_limit_config_count INTEGER NOT NULL,
	rate_limit_config_duration INTEGER NOT NULL,

	created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
	deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS convoy.api_keys (
	id CHAR(26) PRIMARY KEY,

	name TEXT NOT NULL,
	key_type TEXT NOT NULL,
	mask_id TEXT NOT NULL,
	role_type TEXT NOT NULL,
    role_project TEXT NOT NULL,
    role_endpoint TEXT NOT NULL,
	hash TEXT NOT NULL,
	salt TEXT NOT NULL,

	user_id CHAR(26) REFERENCES convoy.users (id),

	created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
	expires_at TIMESTAMPTZ NOT NULL,
	deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS convoy.events (
	id CHAR(26) PRIMARY KEY,

	event_type TEXT NOT NULL,
	endpoints TEXT[] NOT NULL,

	project_id CHAR(26) NOT NULL REFERENCES convoy.projects (id),
	source_id CHAR(26) NOT NULL REFERENCES convoy.sources (id),

	headers JSONB NOT NULL,

	raw TEXT NOT NULL,
	data BYTEA NOT NULL,

	created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
	deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS convoy.event_deliveries (
	id CHAR(26) PRIMARY KEY,

	name TEXT NOT NULL,
	status TEXT NOT NULL,
	description TEXT NOT NULL,

	project_id CHAR(26) NOT NULL REFERENCES convoy.projects (id),
	endpoint_id CHAR(26) REFERENCES convoy.endpoints (id),
	event_id CHAR(26) NOT NULL REFERENCES convoy.events (id),
	device_id CHAR(26) REFERENCES convoy.devices (id),
	subscription_id CHAR(26) NOT NULL REFERENCES convoy.subscriptions (id),

	headers JSONB NOT NULL,
    attempts JSONB,

    metadata JSONB NOT NULL,
    cli_metadata JSONB,

	created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
	deleted_at TIMESTAMPTZ
);

-- +migrate Down
DROP SCHEMA convoy CASCADE;
