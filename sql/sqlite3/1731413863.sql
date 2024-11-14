-- +migrate Up
-- configurations
create table if not exists configurations
(
    id                               TEXT                                       not null,
    is_analytics_enabled             TEXT                                          not null,
    is_signup_enabled                BOOLEAN                                       not null,
    storage_policy_type              TEXT                                          not null,
    on_prem_path                     TEXT,
    s3_bucket                        TEXT,
    s3_access_key                    TEXT,
    s3_secret_key                    TEXT,
    s3_region                        TEXT,
    s3_session_token                 TEXT,
    s3_endpoint                      TEXT,
    created_at                       TEXT not null default (strftime('%Y-%m-%dT%H:%M:%fZ')),
    updated_at                       TEXT not null default (strftime('%Y-%m-%dT%H:%M:%fZ')),
    deleted_at                       TEXT,
    s3_prefix                        TEXT,
    retention_policy_enabled         BOOLEAN                  default false        not null,
    retention_policy_policy          TEXT                     default '720h'       not null,
    cb_minimum_request_count         INTEGER                  default 10           not null,
    cb_sample_rate                   INTEGER                  default 30           not null,
    cb_error_timeout                 INTEGER                  default 30           not null,
    cb_failure_threshold             INTEGER                  default 70           not null,
    cb_success_threshold             INTEGER                  default 1            not null,
    cb_observability_window          INTEGER                  default 30           not null,
    cb_consecutive_failure_threshold INTEGER                  default 10           not null
);

-- event endpoints
create table if not exists events_endpoints
(
    event_id    TEXT not null,
    endpoint_id TEXT not null,
    FOREIGN KEY(endpoint_id) REFERENCES endpoints(id),
    FOREIGN KEY(event_id) REFERENCES events(id)
);

create index if not exists events_endpoints_temp_endpoint_id_idx
    on events_endpoints (endpoint_id);

create unique index if not exists events_endpoints_temp_event_id_endpoint_id_idx1
    on events_endpoints (event_id, endpoint_id);

create index if not exists events_endpoints_temp_event_id_idx
    on events_endpoints (event_id);

create unique index if not exists idx_uq_constraint_events_endpoints_event_id_endpoint_id
    on events_endpoints (event_id, endpoint_id);

-- migrations
create table if not exists gorp_migrations
(
    id         TEXT not null primary key,
    applied_at TEXT
);

-- project configurations
create table if not exists project_configurations
(
    id                                TEXT not null primary key,
    max_payload_read_size             INTEGER not null,
    replay_attacks_prevention_enabled BOOLEAN not null,
    ratelimit_count                   INTEGER not null,
    ratelimit_duration                INTEGER not null,
    strategy_type                     TEXT not null,
    strategy_duration                 INTEGER not null,
    strategy_retry_count              INTEGER not null,
    signature_header                  TEXT not null,
    signature_versions                TEXT not null,
    disable_endpoint                  BOOLEAN default false not null,
    meta_events_enabled               BOOLEAN default false not null,
    meta_events_type                  TEXT,
    meta_events_event_type            TEXT,
    meta_events_url                   TEXT,
    meta_events_secret                TEXT,
    meta_events_pub_sub               TEXT,
    search_policy                     TEXT default '720h',
    multiple_endpoint_subscriptions   BOOLEAN default false not null,
    ssl_enforce_secure_endpoints      BOOLEAN default true,
    created_at                        TEXT not null default (strftime('%Y-%m-%dT%H:%M:%fZ')),
    updated_at                        TEXT not null default (strftime('%Y-%m-%dT%H:%M:%fZ')),
    deleted_at                        TEXT
);

-- source verifiers
create table if not exists source_verifiers
(
    id                      TEXT not null primary key,
    type                    TEXT not null,
    basic_username          TEXT,
    basic_password          TEXT,
    api_key_header_name     TEXT,
    api_key_header_value    TEXT,
    hmac_hash               TEXT,
    hmac_header             TEXT,
    hmac_secret             TEXT,
    hmac_encoding           TEXT,
    twitter_crc_verified_at TEXT,
    created_at              TEXT not null default (strftime('%Y-%m-%dT%H:%M:%fZ')),
    updated_at              TEXT not null default (strftime('%Y-%m-%dT%H:%M:%fZ')),
    deleted_at              TEXT
);

-- token bucket
create table if not exists token_bucket
(
    key        TEXT not null primary key,
    rate       INTEGER not null,
    tokens     INTEGER default 1,
    created_at TEXT not null default (strftime('%Y-%m-%dT%H:%M:%fZ')),
    updated_at TEXT not null default (strftime('%Y-%m-%dT%H:%M:%fZ')),
    expires_at TEXT not null
);

-- users
create table if not exists users
(
    id                            TEXT not null primary key,
    first_name                    TEXT not null,
    last_name                     TEXT not null,
    email                         TEXT not null,
    password                      TEXT not null,
    email_verified                BOOLEAN not null,
    reset_password_token          TEXT,
    email_verification_token      TEXT,
    reset_password_expires_at     TEXT,
    email_verification_expires_at TEXT,
    auth_type                     TEXT default 'local' not null,
    created_at                    TEXT not null default (strftime('%Y-%m-%dT%H:%M:%fZ')),
    updated_at                    TEXT not null default (strftime('%Y-%m-%dT%H:%M:%fZ')),
    deleted_at                    TEXT
);

CREATE UNIQUE INDEX if not exists idx_unique_email_deleted_at
    ON users(email)
    WHERE deleted_at IS NULL;

-- organisations
create table if not exists organisations
(
    id              TEXT not null primary key,
    name            TEXT not null,
    owner_id        TEXT not null,
    custom_domain   TEXT,
    assigned_domain TEXT,
    created_at      TEXT not null default (strftime('%Y-%m-%dT%H:%M:%fZ')),
    updated_at      TEXT not null default (strftime('%Y-%m-%dT%H:%M:%fZ')),
    deleted_at      TEXT,
    FOREIGN KEY(owner_id) REFERENCES users(id)
);

create unique index if not exists idx_organisations_custom_domain_deleted_at
    on organisations (custom_domain, assigned_domain)
    where (deleted_at IS NULL);

--projects
create table if not exists projects
(
    id                       TEXT not null primary key,
    name                     TEXT not null,
    type                     TEXT not null,
    logo_url                 TEXT,
    retained_events          INTEGER default 0,
    organisation_id          TEXT not null,
    project_configuration_id TEXT not null,
    created_at               TEXT not null default (strftime('%Y-%m-%dT%H:%M:%fZ')),
    updated_at               TEXT not null default (strftime('%Y-%m-%dT%H:%M:%fZ')),
    deleted_at               TEXT,
    constraint name_org_id_key unique (name, organisation_id, deleted_at),
    FOREIGN KEY(organisation_id) REFERENCES organisations(id),
    FOREIGN KEY(project_configuration_id) REFERENCES project_configurations(id)
);

create unique index if not exists idx_name_organisation_id_deleted_at
    on projects (organisation_id, name)
    where (deleted_at IS NULL);

-- applications todo(raymond): deprecate me
create table if not exists applications
(
    id                TEXT not null primary key,
    project_id        TEXT not null,
    title             TEXT not null,
    support_email     TEXT,
    slack_webhook_url TEXT,
    created_at        TEXT not null default (strftime('%Y-%m-%dT%H:%M:%fZ')),
    updated_at        TEXT not null default (strftime('%Y-%m-%dT%H:%M:%fZ')),
    deleted_at        TEXT,
    FOREIGN KEY(project_id) REFERENCES projects(id)
);

--  devices todo(raymond): deprecate me
create table if not exists devices
(
    id           TEXT not null primary key,
    project_id   TEXT not null,
    host_name    TEXT not null,
    status       TEXT not null,
    created_at   TEXT not null default (strftime('%Y-%m-%dT%H:%M:%fZ')),
    updated_at   TEXT not null default (strftime('%Y-%m-%dT%H:%M:%fZ')),
    last_seen_at TEXT not null,
    deleted_at   TEXT,
    FOREIGN KEY(project_id) REFERENCES projects(id)
);

-- endpoints
create table if not exists endpoints
(
    id                                       TEXT not null primary key,
    status                                   TEXT not null,
    owner_id                                 TEXT,
    description                              TEXT,
    rate_limit                               INTEGER not null,
    advanced_signatures                      BOOLEAN not null,
    slack_webhook_url                        TEXT,
    support_email                            TEXT,
    app_id                                   TEXT,
    project_id                               TEXT not null,
    authentication_type                      TEXT,
    authentication_type_api_key_header_name  TEXT,
    authentication_type_api_key_header_value TEXT,
    secrets                                  TEXT not null,
    created_at                               TEXT not null default (strftime('%Y-%m-%dT%H:%M:%fZ')),
    updated_at                               TEXT not null default (strftime('%Y-%m-%dT%H:%M:%fZ')),
    deleted_at                               TEXT,
    http_timeout                             INTEGER not null,
    rate_limit_duration                      INTEGER not null,
    name                                     TEXT not null,
    url                                      TEXT not null,
    FOREIGN KEY(project_id) REFERENCES projects(id)
);

CREATE UNIQUE INDEX if not exists idx_name_project_id_deleted_at
    ON endpoints(name, project_id)
    WHERE deleted_at IS NULL;

create index if not exists idx_endpoints_name_key
    on endpoints (name);

create index if not exists idx_endpoints_app_id_key
    on endpoints (app_id);

create index if not exists idx_endpoints_owner_id_key
    on endpoints (owner_id);

create index if not exists idx_endpoints_project_id_key
    on endpoints (project_id);

-- api keys
create table if not exists api_keys
(
    id            TEXT not null primary key,
    name          TEXT    not null,
    key_type      TEXT    not null,
    mask_id       TEXT    not null,
    role_type     TEXT,
    role_project  TEXT,
    role_endpoint TEXT,
    hash          TEXT    not null,
    salt          TEXT    not null,
    user_id       TEXT,
    created_at    TEXT not null default (strftime('%Y-%m-%dT%H:%M:%fZ')),
    updated_at    TEXT not null default (strftime('%Y-%m-%dT%H:%M:%fZ')),
    expires_at    TEXT,
    deleted_at    TEXT,
    FOREIGN KEY(role_project) REFERENCES projects(id),
    FOREIGN KEY(role_endpoint) REFERENCES endpoints(id),
    FOREIGN KEY(user_id) REFERENCES users(id)
);

CREATE UNIQUE INDEX if not exists idx_mask_id_deleted_at
    ON api_keys(mask_id)
    WHERE deleted_at IS NULL;

create index if not exists idx_api_keys_mask_id
    on api_keys (mask_id);

-- event types
create table if not exists event_types
(
    id            TEXT not null primary key,
    name          TEXT not null,
    description   TEXT,
    project_id    TEXT not null,
    category      TEXT,
    created_at    TEXT default (strftime('%Y-%m-%dT%H:%M:%fZ')) not null,
    updated_at    TEXT default (strftime('%Y-%m-%dT%H:%M:%fZ')) not null,
    deprecated_at TEXT,
    FOREIGN KEY(project_id) REFERENCES projects(id)
);

CREATE UNIQUE INDEX if not exists idx_name_project_id_deleted_at
    ON event_types(name, project_id)
    WHERE deprecated_at IS NULL;

create index if not exists idx_event_types_category
    on event_types (category);

create index if not exists idx_event_types_category_deprecated
    on event_types (category)
    where (deprecated_at IS NOT NULL);

create index if not exists idx_event_types_category_not_deprecated
    on event_types (category)
    where (deprecated_at IS NULL);

create index if not exists idx_event_types_name
    on event_types (name);

create index if not exists idx_event_types_name_deprecated
    on event_types (name)
    where (deprecated_at IS NOT NULL);

create index if not exists idx_event_types_name_not_deprecated
    on event_types (name)
    where (deprecated_at IS NULL);


-- jobs
create table if not exists jobs
(
    id           TEXT not null primary key,
    type         TEXT not null,
    status       TEXT not null,
    project_id   TEXT not null,
    started_at   TEXT,
    completed_at TEXT,
    failed_at    TEXT,
    created_at   TEXT not null default (strftime('%Y-%m-%dT%H:%M:%fZ')),
    updated_at   TEXT not null default (strftime('%Y-%m-%dT%H:%M:%fZ')),
    deleted_at   TEXT,
    FOREIGN KEY(project_id) REFERENCES projects(id)
);

-- meta events
create table if not exists meta_events
(
    id         TEXT not null primary key,
    event_type TEXT not null,
    project_id TEXT not null,
    metadata   TEXT not null,
    attempt    TEXT,
    status     TEXT not null,
    created_at TEXT not null default (strftime('%Y-%m-%dT%H:%M:%fZ')),
    updated_at TEXT not null default (strftime('%Y-%m-%dT%H:%M:%fZ')),
    deleted_at TEXT,
    FOREIGN KEY(project_id) REFERENCES projects(id)
);

-- organisation invites
create table if not exists organisation_invites
(
    id              TEXT not null primary key,
    organisation_id TEXT not null,
    invitee_email   TEXT not null,
    token           TEXT not null,
    role_type       TEXT not null,
    role_project    TEXT,
    role_endpoint   TEXT,
    status          TEXT not null,
    created_at      TEXT not null default (strftime('%Y-%m-%dT%H:%M:%fZ')),
    updated_at      TEXT not null default (strftime('%Y-%m-%dT%H:%M:%fZ')),
    expires_at      TEXT not null,
    deleted_at      TEXT,
    FOREIGN KEY(role_project) REFERENCES projects(id),
    FOREIGN KEY(organisation_id) REFERENCES organisations(id),
    FOREIGN KEY(role_endpoint) REFERENCES endpoints(id)
);

CREATE UNIQUE INDEX if not exists idx_token_organisation_id_deleted_at
    ON organisation_invites(token, organisation_id)
    WHERE deleted_at IS NULL;

create index if not exists idx_organisation_invites_token_key
    on organisation_invites (token);

create unique index if not exists organisation_invites_invitee_email
    on organisation_invites (organisation_id, invitee_email, deleted_at);

-- organisation members
create table if not exists organisation_members
(
    id              TEXT not null primary key,
    role_type       TEXT not null,
    role_project    TEXT,
    role_endpoint   TEXT,
    user_id         TEXT not null,
    organisation_id TEXT not null,
    created_at      TEXT not null default (strftime('%Y-%m-%dT%H:%M:%fZ')),
    updated_at      TEXT not null default (strftime('%Y-%m-%dT%H:%M:%fZ')),
    deleted_at      TEXT,
    FOREIGN KEY(role_project) REFERENCES projects(id),
    FOREIGN KEY(role_endpoint) REFERENCES endpoints(id),
    FOREIGN KEY(organisation_id) REFERENCES organisations(id),
    FOREIGN KEY(user_id) REFERENCES users(id)
);

CREATE UNIQUE INDEX if not exists idx_organisation_id_user_id_deleted_at
    ON organisation_members(organisation_id, user_id)
    WHERE deleted_at IS NULL;

create index if not exists idx_organisation_members_deleted_at_key
    on organisation_members (deleted_at);

create index if not exists idx_organisation_members_organisation_id_key
    on organisation_members (organisation_id);

create index if not exists idx_organisation_members_user_id_key
    on organisation_members (user_id);

create table if not exists portal_links
(
    id                  TEXT not null primary key,
    project_id          TEXT not null,
    name                TEXT not null,
    token               TEXT not null,
    endpoints           TEXT,
    created_at          TEXT not null default (strftime('%Y-%m-%dT%H:%M:%fZ')),
    updated_at          TEXT not null default (strftime('%Y-%m-%dT%H:%M:%fZ')),
    deleted_at          TEXT,
    owner_id            TEXT,
    can_manage_endpoint BOOLEAN default false,
    FOREIGN KEY(project_id) REFERENCES projects(id)
);

CREATE UNIQUE INDEX if not exists idx_token_deleted_at
    ON portal_links(token)
    WHERE deleted_at IS NULL;

create index if not exists idx_portal_links_owner_id_key
    on portal_links (owner_id);

create index if not exists idx_portal_links_project_id
    on portal_links (project_id);

create index if not exists idx_portal_links_token
    on portal_links (token);

create table if not exists portal_links_endpoints
(
    portal_link_id TEXT not null,
    endpoint_id    TEXT not null,
    FOREIGN KEY(portal_link_id) REFERENCES portal_links(id),
    FOREIGN KEY(endpoint_id) REFERENCES endpoints(id)
);

create index if not exists idx_portal_links_endpoints_enpdoint_id
    on portal_links_endpoints (endpoint_id);

create index if not exists idx_portal_links_endpoints_portal_link_id
    on portal_links_endpoints (portal_link_id);


-- sources
create table if not exists sources
(
    id                           TEXT not null primary key,
    name                         TEXT not null,
    type                         TEXT not null,
    mask_id                      TEXT not null,
    provider                     TEXT not null,
    is_disabled                  BOOLEAN default false,
    forward_headers              TEXT[],
    project_id                   TEXT not null,
    source_verifier_id           TEXT,
    pub_sub                      TEXT,
    deleted_at                   TEXT,
    custom_response_body         TEXT,
    custom_response_content_type TEXT,
    idempotency_keys             TEXT[],
    body_function                TEXT,
    header_function              TEXT,
    created_at                   TEXT not null default (strftime('%Y-%m-%dT%H:%M:%fZ')),
    updated_at                   TEXT not null default (strftime('%Y-%m-%dT%H:%M:%fZ')),
    FOREIGN KEY(project_id) REFERENCES projects(id),
    FOREIGN KEY(source_verifier_id) REFERENCES source_verifiers(id)
);

CREATE UNIQUE INDEX if not exists idx_mask_id_project_id_deleted_at
    ON sources(mask_id, project_id)
    WHERE deleted_at IS NULL;

create index if not exists idx_sources_mask_id
    on sources (mask_id);

create index if not exists idx_sources_project_id
    on sources (project_id);

create index if not exists idx_sources_source_verifier_id
    on sources (source_verifier_id);

-- events
create table if not exists events
(
    id                 TEXT not null primary key,
    event_type         TEXT not null,
    data               TEXT not null,
    project_id         TEXT not null,
    raw                TEXT not null,
    endpoints          TEXT,
    source_id          TEXT,
    headers            TEXT,
    deleted_at         TEXT,
    url_query_params   TEXT,
    idempotency_key    TEXT,
    acknowledged_at    TEXT,
    status             TEXT,
    metadata           TEXT,
    is_duplicate_event BOOLEAN default false,
    created_at         TEXT not null default (strftime('%Y-%m-%dT%H:%M:%fZ')),
    updated_at         TEXT not null default (strftime('%Y-%m-%dT%H:%M:%fZ')),
    FOREIGN KEY(source_id) REFERENCES sources(id),
    FOREIGN KEY(project_id) REFERENCES projects(name)
);

create index if not exists idx_events_created_at_key
    on events (created_at);

create index if not exists idx_events_deleted_at_key
    on events (deleted_at);

create index if not exists idx_events_project_id_deleted_at_key
    on events (project_id, deleted_at);

create index if not exists idx_events_project_id_key
    on events (project_id);

create index if not exists idx_events_project_id_source_id
    on events (project_id, source_id);

create index if not exists idx_events_source_id
    on events (source_id);

create index if not exists idx_events_source_id_key
    on events (source_id);

create index if not exists idx_idempotency_key_key
    on events (idempotency_key);

create index if not exists idx_project_id_on_not_deleted
    on events (project_id)
    where (deleted_at IS NULL);

-- subscriptions
create table if not exists subscriptions
(
    id                                TEXT not null primary key,
    name                              TEXT    not null,
    type                              TEXT    not null,
    project_id                        TEXT not null,
    endpoint_id                       TEXT,
    device_id                         TEXT,
    source_id                         TEXT,
    alert_config_count                INTEGER not null,
    alert_config_threshold            TEXT    not null,
    retry_config_type                 TEXT    not null,
    retry_config_duration             INTEGER not null,
    retry_config_retry_count          INTEGER not null,
    filter_config_event_types         TEXT[]  not null,
    filter_config_filter_headers      TEXT   not null,
    filter_config_filter_body         TEXT   not null,
    rate_limit_config_count           INTEGER not null,
    rate_limit_config_duration        INTEGER not null,
    created_at                        TEXT not null default (strftime('%Y-%m-%dT%H:%M:%fZ')),
    updated_at                        TEXT not null default (strftime('%Y-%m-%dT%H:%M:%fZ')),
    deleted_at                        TEXT,
    function                          TEXT,
    filter_config_filter_is_flattened BOOLEAN default false,
    FOREIGN KEY(source_id) REFERENCES sources(id),
    FOREIGN KEY(device_id) REFERENCES devices(id),
    FOREIGN KEY(endpoint_id) REFERENCES endpoints(id),
    FOREIGN KEY(project_id) REFERENCES projects(id)
);

CREATE UNIQUE INDEX if not exists idx_name_project_id_deleted_at
    ON subscriptions(name, project_id)
    WHERE deleted_at IS NULL;

create index if not exists idx_subscriptions_filter_config_event_types_key
    on subscriptions (filter_config_event_types);

create index if not exists idx_subscriptions_id_deleted_at
    on subscriptions (id, deleted_at)
    where (deleted_at IS NOT NULL);

create index if not exists idx_subscriptions_name_key
    on subscriptions (name)
    where (deleted_at IS NULL);

create index if not exists idx_subscriptions_updated_at
    on subscriptions (updated_at)
    where (deleted_at IS NULL);

create index if not exists idx_subscriptions_updated_at_id_project_id
    on subscriptions (updated_at, id, project_id)
    where (deleted_at IS NULL);

-- event deliveries
create table if not exists event_deliveries
(
    id               TEXT not null primary key,
    status           TEXT not null,
    description      TEXT not null,
    project_id       TEXT not null,
    endpoint_id      TEXT,
    event_id         TEXT not null,
    device_id        TEXT,
    subscription_id  TEXT not null,
    metadata         TEXT not null,
    headers          TEXT,
    attempts         TEXT,
    cli_metadata     TEXT,
    url_query_params TEXT,
    idempotency_key  TEXT,
    latency          TEXT,
    event_type       TEXT,
    acknowledged_at  TEXT,
    latency_seconds  NUMERIC,
    created_at       TEXT not null default (strftime('%Y-%m-%dT%H:%M:%fZ')),
    updated_at       TEXT not null default (strftime('%Y-%m-%dT%H:%M:%fZ')),
    FOREIGN KEY(subscription_id) REFERENCES subscriptions(id),
    FOREIGN KEY(device_id) REFERENCES devices(id),
    FOREIGN KEY(event_id) REFERENCES events(id),
    FOREIGN KEY(endpoint_id) REFERENCES endpoints(id),
    FOREIGN KEY(project_id) REFERENCES projects(id)
);

-- delivery attempts
create table if not exists delivery_attempts
(
    id                   TEXT not null primary key,
    url                  TEXT not null,
    method               TEXT not null,
    api_version          TEXT not null,
    project_id           TEXT not null,
    endpoint_id          TEXT not null,
    event_delivery_id    TEXT not null,
    ip_address           TEXT,
    request_http_header  TEXT,
    response_http_header TEXT,
    http_status          TEXT,
    response_data        TEXT,
    error                TEXT,
    status               BOOLEAN,
    created_at           TEXT not null default (strftime('%Y-%m-%dT%H:%M:%fZ')),
    updated_at           TEXT not null default (strftime('%Y-%m-%dT%H:%M:%fZ')),
    deleted_at           TEXT,
    FOREIGN KEY(event_delivery_id) REFERENCES event_deliveries(id),
    FOREIGN KEY(endpoint_id) REFERENCES endpoints(id),
    FOREIGN KEY(project_id) REFERENCES projects(id)
);

create index if not exists idx_delivery_attempts_created_at
    on delivery_attempts (created_at);

create index if not exists idx_delivery_attempts_created_at_id_event_delivery_id
    on delivery_attempts (created_at, id, project_id, event_delivery_id)
    where (deleted_at IS NULL);

create index if not exists idx_delivery_attempts_event_delivery_id
    on delivery_attempts (event_delivery_id);

create index if not exists idx_delivery_attempts_event_delivery_id_created_at
    on delivery_attempts (event_delivery_id, created_at);

create index if not exists idx_delivery_attempts_event_delivery_id_created_at_desc
    on delivery_attempts (event_delivery_id asc, created_at desc);

create index if not exists event_deliveries_event_type_1
    on event_deliveries (event_type);

create index if not exists idx_event_deliveries_created_at_key
    on event_deliveries (created_at);

create index if not exists idx_event_deliveries_device_id_key
    on event_deliveries (device_id);

create index if not exists idx_event_deliveries_endpoint_id_key
    on event_deliveries (endpoint_id);

create index if not exists idx_event_deliveries_event_id_key
    on event_deliveries (event_id);

create index if not exists idx_event_deliveries_project_id_endpoint_id
    on event_deliveries (project_id, endpoint_id);

create index if not exists idx_event_deliveries_project_id_endpoint_id_status
    on event_deliveries (project_id, endpoint_id, status);

create index if not exists idx_event_deliveries_project_id_event_id
    on event_deliveries (project_id, event_id);

create index if not exists idx_event_deliveries_project_id_key
    on event_deliveries (project_id);

create index if not exists idx_event_deliveries_status
    on event_deliveries (status);

create index if not exists idx_event_deliveries_status_key
    on event_deliveries (status);

-- +migrate Down
drop table if exists configurations;
drop table if exists events_endpoints;
drop table if exists gorp_migrations;
drop table if exists project_configurations;
drop table if exists source_verifiers;
drop table if exists token_bucket;
drop table if exists users;
drop table if exists organisations;
drop table if exists projects;
drop table if exists applications;
drop table if exists devices;
drop table if exists endpoints;
drop table if exists api_keys;
drop table if exists event_types;
drop table if exists jobs;
drop table if exists meta_events;
drop table if exists organisation_invites;
drop table if exists organisation_members;
drop table if exists portal_links;
drop table if exists portal_links_endpoints;
drop table if exists sources;
drop table if exists events;
drop table if exists events_search;
drop table if exists subscriptions;
drop table if exists event_deliveries;
drop table if exists delivery_attempts;