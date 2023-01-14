-- +migrate Up
CREATE SCHEMA IF NOT EXISTS convoy;

CREATE TABLE IF NOT EXISTS convoy.organisations (
	id TEXT UNIQUE NOT NULL,
	
	name TEXT NOT NULL,
	owner_id TEXT NOT NULL,
	custom_domain TEXT NOT NULL,
	assigned_domain TEXT NOT NULL,

	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	deleted_at TIMESTAMP,

    PRIMARY KEY (id, deleted_at)
);

CREATE TABLE IF NOT EXISTS convoy.users (
	id TEXT UNIQUE NOT NULL,
	
	first_name TEXT NOT NULL,
	last_name TEXT NOT NULL,
	role TEXT NOT NULL,
	email TEXT NOT NULL,
	password TEXT NOT NULL,
	email_verified BOOL NOT NULL,
	reset_password_token TEXT NOT NULL,
	email_verification_token TEXT NOT NULL,

	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	deleted_at TIMESTAMP,
	reset_password_expires_at TIMESTAMP,
	email_verification_expires_at TIMESTAMP,

    PRIMARY KEY (id, deleted_at)
);

CREATE TABLE IF NOT EXISTS convoy.organisation_members (
	id TEXT UNIQUE NOT NULL,
	
	role TEXT NOT NULL,
	user_metadata JSONB NOT NULL,
	user_id TEXT NOT NULL REFERENCES convoy.users (id),
	organisation_id TEXT NOT NULL REFERENCES convoy.organisations (id),

	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	deleted_at TIMESTAMP,

    PRIMARY KEY (id, deleted_at)
);

CREATE TABLE IF NOT EXISTS convoy.projects (
	id TEXT UNIQUE NOT NULL,
	
	name TEXT NOT NULL,
	type TEXT NOT NULL,
	logo_url TEXT NOT NULL,
	metadata JSONB NOT NULL,
	config JSONB NOT NULL,
	rate_limit INTEGER NOT NULL,
	rate_limit_duration TEXT NOT NULL,
	organisation_id TEXT NOT NULL REFERENCES convoy.organisations (id),
	
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	deleted_at TIMESTAMP,

    PRIMARY KEY (id, deleted_at)
);

CREATE TABLE IF NOT EXISTS convoy.endpoints (
	id TEXT UNIQUE NOT NULL,
	
	project_id TEXT NOT NULL REFERENCES convoy.projects (id),
	owner_id TEXT NOT NULL,
	target_url TEXT NOT NULL,
	title TEXT NOT NULL,
	description TEXT NOT NULL,
	secrets JSONB NOT NULL,
	advanced_signatures JSONB NOT NULL, 
	authentication JSONB NOT NULL,
	slack_webhook_url TEXT,
	support_email TEXT,
	app_id TEXT,
	
	http_timeout TEXT NOT NULL,
	rate_limit INTEGER NOT NULL,
	rate_limit_duration TEXT NOT NULL,
	status TEXT NOT NULL,
	
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	deleted_at TIMESTAMP,
	
    PRIMARY KEY (id, deleted_at)
);

CREATE TABLE IF NOT EXISTS convoy.applications (
	id TEXT UNIQUE NOT NULL,
	
	project_id TEXT NOT NULL REFERENCES convoy.projects (id),
	title TEXT NOT NULL,
	support_email TEXT,
	slack_webhook_url TEXT,
	endpoints JSONB NOT NULL,
	
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	deleted_at TIMESTAMP,
	
    PRIMARY KEY (id, deleted_at)
);

CREATE TABLE IF NOT EXISTS convoy.organisation_invites (
	id TEXT UNIQUE NOT NULL,
	
	organisation_id TEXT NOT NULL REFERENCES convoy.organisations (id),
	invitee_email TEXT NOT NULL,
	token TEXT NOT NULL,
	role TEXT NOT NULL,
	status TEXT NOT NULL,
	
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	expires_at TIMESTAMP NOT NULL,
	deleted_at TIMESTAMP,
	
    PRIMARY KEY (id, deleted_at)
);

CREATE TABLE IF NOT EXISTS convoy.portal_links (
	id TEXT UNIQUE NOT NULL,

	project_id TEXT NOT NULL REFERENCES convoy.projects (id),
	
	invitee_email TEXT NOT NULL,
	token TEXT NOT NULL,
	role TEXT NOT NULL,
	status TEXT NOT NULL,
	
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	expires_at TIMESTAMP NOT NULL,
	deleted_at TIMESTAMP,
	
    PRIMARY KEY (id, deleted_at)
);

CREATE TABLE IF NOT EXISTS convoy.subscription_filters (
	id TEXT UNIQUE PRIMARY KEY,
	invitee_email JSONB NOT NULL,	
	deleted_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS convoy.devices (
	id TEXT UNIQUE NOT NULL,

	project_id TEXT NOT NULL REFERENCES convoy.projects (id),
	endpoint_id TEXT NOT NULL REFERENCES convoy.endpoints (id),
	
	host_name TEXT NOT NULL,
	token TEXT NOT NULL,
	status TEXT NOT NULL,
	
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	last_seen_at TIMESTAMP NOT NULL,
	deleted_at TIMESTAMP,
	
    PRIMARY KEY (id, deleted_at)
);

CREATE TABLE IF NOT EXISTS convoy.configurations (
	id TEXT UNIQUE NOT NULL,
	
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
	
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	deleted_at TIMESTAMP,
	
    PRIMARY KEY (id, deleted_at)
);

CREATE TABLE IF NOT EXISTS convoy.sources (
	id TEXT UNIQUE NOT NULL,
	
	name TEXT NOT NULL,
	type TEXT NOT NULL,
	mask_id TEXT NOT NULL,
	provider TEXT NOT NULL,
	is_disabled BOOLEAN NOT NULL,
	forward_headers TEXT[],
	verifier JSONB,
	provider_config JSONB,
	
	project_id TEXT NOT NULL REFERENCES convoy.projects (id),
		
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	deleted_at TIMESTAMP,
	
    PRIMARY KEY (id, deleted_at)
);

CREATE TABLE IF NOT EXISTS convoy.subscriptions (
	id TEXT UNIQUE NOT NULL,
	
	name TEXT NOT NULL,
	type TEXT NOT NULL,

	project_id TEXT NOT NULL REFERENCES convoy.projects (id),
	endpoint_id TEXT NOT NULL REFERENCES convoy.endpoints (id),
	device_id TEXT NOT NULL REFERENCES convoy.devices (id),
	source_id TEXT NOT NULL REFERENCES convoy.sources (id),
	
	alert_config JSONB NOT NULL,
	retry_config JSONB NOT NULL,
	filter_config JSONB NOT NULL,
	rate_limit_config JSONB NOT NULL,
	
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	deleted_at TIMESTAMP,
	
    PRIMARY KEY (id, deleted_at)
);

CREATE TABLE IF NOT EXISTS convoy.api_keys (
	id TEXT UNIQUE NOT NULL,
	
	name TEXT NOT NULL,
	key_type TEXT NOT NULL,
	mask_id TEXT NOT NULL,
	role TEXT NOT NULL,
	hash TEXT NOT NULL,
	salt TEXT NOT NULL,

	user_id TEXT NOT NULL REFERENCES convoy.users (id),
	
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	expires_at TIMESTAMP NOT NULL,
	deleted_at TIMESTAMP,
	
    PRIMARY KEY (id, deleted_at)
);

CREATE TABLE IF NOT EXISTS convoy.events (
	id TEXT UNIQUE NOT NULL,
	
	event_type TEXT NOT NULL,
	endpoints TEXT[] NOT NULL,
	
	project_id TEXT NOT NULL REFERENCES convoy.projects (id),
	source_id TEXT NOT NULL REFERENCES convoy.sources (id),
	
	headers JSONB NOT NULL,
	endpoint_metadata JSONB NOT NULL,
	
	-- Data is an arbitrary JSON value that
	-- gets sent as the body of the
	-- webhook to the endpoints
	raw TEXT NOT NULL,
	data BYTEA NOT NULL,
	
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	deleted_at TIMESTAMP,
	
    PRIMARY KEY (id, deleted_at)
);

CREATE TABLE IF NOT EXISTS convoy.event_eliveries (
	id TEXT UNIQUE NOT NULL,
	
	name TEXT NOT NULL,
	status TEXT NOT NULL,
	description TEXT NOT NULL,

	project_id TEXT NOT NULL REFERENCES convoy.projects (id),
	endpoint_id TEXT NOT NULL REFERENCES convoy.endpoints (id),
	event_id TEXT NOT NULL REFERENCES convoy.events (id),
	device_id TEXT NOT NULL REFERENCES convoy.devices (id),
	subscription_id TEXT NOT NULL REFERENCES convoy.subscriptions (id),
	
	headers JSONB NOT NULL,
	metadata JSONB NOT NULL,
	cli_metadata JSONB,
	
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	deleted_at TIMESTAMP,
	
    PRIMARY KEY (id, deleted_at)
);

CREATE TABLE IF NOT EXISTS convoy.delivery_attempts (
	id TEXT UNIQUE NOT NULL,
	
	msg_id TEXT NOT NULL,
	url TEXT NOT NULL,
	method TEXT NOT NULL,
	api_version TEXT NOT NULL,
	ip_address TEXT NOT NULL,
	http_status TEXT NOT NULL,
	response_data TEXT NOT NULL,
	error TEXT NOT NULL,
	status TEXT NOT NULL,
	
	request_http_header JSONB NOT NULL,
	response_http_header JSONB NOT NULL,

	event_elivery_id TEXT NOT NULL REFERENCES convoy.event_eliveries (id),
	endpoint_id TEXT NOT NULL REFERENCES convoy.endpoints (id),
	
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	expires_at TIMESTAMP NOT NULL,
	deleted_at TIMESTAMP,
	
    PRIMARY KEY (id, deleted_at)
);








-- +migrate Down
DROP TABLE convoy.projects;

DROP SCHEMA convoy;