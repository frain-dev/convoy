-- Project Configuration Queries

-- name: CreateProjectConfiguration :exec
INSERT INTO convoy.project_configurations (
    id, search_policy, max_payload_read_size,
    replay_attacks_prevention_enabled, ratelimit_count,
    ratelimit_duration, strategy_type, strategy_duration,
    strategy_retry_count, signature_header, signature_versions,
    disable_endpoint, meta_events_enabled, meta_events_type,
    meta_events_event_type, meta_events_url, meta_events_secret,
    meta_events_pub_sub, ssl_enforce_secure_endpoints,
    cb_sample_rate, cb_error_timeout, cb_failure_threshold,
    cb_success_threshold, cb_observability_window,
    cb_minimum_request_count, cb_consecutive_failure_threshold
)
VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13,
    $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26
);

-- name: UpdateProjectConfiguration :execresult
UPDATE convoy.project_configurations SET
    max_payload_read_size = $2,
    replay_attacks_prevention_enabled = $3,
    ratelimit_count = $4,
    ratelimit_duration = $5,
    strategy_type = $6,
    strategy_duration = $7,
    strategy_retry_count = $8,
    signature_header = $9,
    signature_versions = $10,
    disable_endpoint = $11,
    meta_events_enabled = $12,
    meta_events_type = $13,
    meta_events_event_type = $14,
    meta_events_url = $15,
    meta_events_secret = $16,
    meta_events_pub_sub = $17,
    search_policy = $18,
    ssl_enforce_secure_endpoints = $19,
    cb_sample_rate = $20,
    cb_error_timeout = $21,
    cb_failure_threshold = $22,
    cb_success_threshold = $23,
    cb_observability_window = $24,
    cb_minimum_request_count = $25,
    cb_consecutive_failure_threshold = $26,
    updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL;

-- Project CRUD Queries

-- name: CreateProject :exec
INSERT INTO convoy.projects (id, name, type, logo_url, organisation_id, project_configuration_id)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: FetchProjectByID :one
SELECT
    p.id,
    p.name,
    p.type,
    p.retained_events,
    p.logo_url,
    p.organisation_id,
    p.project_configuration_id,
    c.search_policy AS "config_search_policy",
    c.max_payload_read_size AS "config_max_payload_read_size",
    c.multiple_endpoint_subscriptions AS "config_multiple_endpoint_subscriptions",
    c.replay_attacks_prevention_enabled AS "config_replay_attacks_prevention_enabled",
    c.ratelimit_count AS "config_ratelimit_count",
    c.ratelimit_duration AS "config_ratelimit_duration",
    c.strategy_type AS "config_strategy_type",
    c.strategy_duration AS "config_strategy_duration",
    c.strategy_retry_count AS "config_strategy_retry_count",
    c.signature_header AS "config_signature_header",
    c.signature_versions AS "config_signature_versions",
    c.disable_endpoint AS "config_disable_endpoint",
    c.ssl_enforce_secure_endpoints AS "config_ssl_enforce_secure_endpoints",
    c.meta_events_enabled AS "config_meta_events_enabled",
    COALESCE(c.meta_events_type, '') AS "config_meta_events_type",
    c.meta_events_event_type AS "config_meta_events_event_type",
    COALESCE(c.meta_events_url, '') AS "config_meta_events_url",
    COALESCE(c.meta_events_secret, '') AS "config_meta_events_secret",
    c.meta_events_pub_sub AS "config_meta_events_pub_sub",
    c.cb_sample_rate AS "config_cb_sample_rate",
    c.cb_error_timeout AS "config_cb_error_timeout",
    c.cb_failure_threshold AS "config_cb_failure_threshold",
    c.cb_success_threshold AS "config_cb_success_threshold",
    c.cb_observability_window AS "config_cb_observability_window",
    c.cb_minimum_request_count AS "config_cb_minimum_request_count",
    c.cb_consecutive_failure_threshold AS "config_cb_consecutive_failure_threshold",
    p.created_at,
    p.updated_at,
    p.deleted_at
FROM convoy.projects p
LEFT JOIN convoy.project_configurations c
ON p.project_configuration_id = c.id
WHERE p.id = $1 AND p.deleted_at IS NULL;

-- name: FetchProjects :many
SELECT
    p.id,
    p.name,
    p.type,
    p.retained_events,
    p.logo_url,
    p.organisation_id,
    p.project_configuration_id,
    c.search_policy AS "config_search_policy",
    c.max_payload_read_size AS "config_max_payload_read_size",
    c.multiple_endpoint_subscriptions AS "config_multiple_endpoint_subscriptions",
    c.replay_attacks_prevention_enabled AS "config_replay_attacks_prevention_enabled",
    c.ratelimit_count AS "config_ratelimit_count",
    c.ratelimit_duration AS "config_ratelimit_duration",
    c.strategy_type AS "config_strategy_type",
    c.strategy_duration AS "config_strategy_duration",
    c.strategy_retry_count AS "config_strategy_retry_count",
    c.signature_header AS "config_signature_header",
    c.signature_versions AS "config_signature_versions",
    c.disable_endpoint AS "config_disable_endpoint",
    c.ssl_enforce_secure_endpoints AS "config_ssl_enforce_secure_endpoints",
    c.meta_events_enabled AS "config_meta_events_enabled",
    COALESCE(c.meta_events_type, '') AS "config_meta_events_type",
    c.meta_events_event_type AS "config_meta_events_event_type",
    COALESCE(c.meta_events_url, '') AS "config_meta_events_url",
    COALESCE(c.meta_events_secret, '') AS "config_meta_events_secret",
    c.meta_events_pub_sub AS "config_meta_events_pub_sub",
    c.cb_sample_rate AS "config_cb_sample_rate",
    c.cb_error_timeout AS "config_cb_error_timeout",
    c.cb_failure_threshold AS "config_cb_failure_threshold",
    c.cb_success_threshold AS "config_cb_success_threshold",
    c.cb_observability_window AS "config_cb_observability_window",
    c.cb_minimum_request_count AS "config_cb_minimum_request_count",
    c.cb_consecutive_failure_threshold AS "config_cb_consecutive_failure_threshold",
    p.created_at,
    p.updated_at,
    p.deleted_at
FROM convoy.projects p
LEFT JOIN convoy.project_configurations c
ON p.project_configuration_id = c.id
WHERE (p.organisation_id = @org_id OR @org_id = '') AND p.deleted_at IS NULL
ORDER BY p.id;

-- name: UpdateProject :execresult
UPDATE convoy.projects SET
    name = $2,
    logo_url = $3,
    retained_events = $4,
    updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL;

-- name: DeleteProject :execresult
UPDATE convoy.projects SET
    deleted_at = NOW()
WHERE id = $1 AND deleted_at IS NULL;

-- Cascade Delete Queries

-- name: DeleteProjectEndpoints :execresult
UPDATE convoy.endpoints SET
    deleted_at = NOW()
WHERE project_id = $1 AND deleted_at IS NULL;

-- name: DeleteProjectEvents :execresult
UPDATE convoy.events
SET deleted_at = NOW()
WHERE project_id = $1 AND deleted_at IS NULL;

-- name: DeleteProjectSubscriptions :execresult
UPDATE convoy.subscriptions SET
    deleted_at = NOW()
WHERE project_id = $1 AND deleted_at IS NULL;

-- Statistics Queries

-- name: FetchProjectStatistics :one
SELECT
    (SELECT EXISTS(SELECT 1 FROM convoy.subscriptions WHERE project_id = $1 AND deleted_at IS NULL)) AS subscriptions_exist,
    (SELECT EXISTS(SELECT 1 FROM convoy.endpoints WHERE project_id = $1 AND deleted_at IS NULL)) AS endpoints_exist,
    (SELECT EXISTS(SELECT 1 FROM convoy.sources WHERE project_id = $1 AND deleted_at IS NULL)) AS sources_exist,
    (SELECT EXISTS(SELECT 1 FROM convoy.events WHERE project_id = $1 AND deleted_at IS NULL)) AS events_exist;

-- Endpoint Update Query

-- name: UpdateProjectEndpointStatus :many
UPDATE convoy.endpoints
SET status = $1, updated_at = NOW()
WHERE project_id = $2
    AND status = ANY($3::text[])
    AND deleted_at IS NULL
RETURNING
    id, name, status, owner_id, url,
    description, http_timeout, rate_limit, rate_limit_duration,
    advanced_signatures, slack_webhook_url, support_email,
    app_id, project_id, secrets, created_at, updated_at,
    authentication_type AS "authentication_type",
    authentication_type_api_key_header_name AS "authentication_api_key_header_name",
    authentication_type_api_key_header_value AS "authentication_api_key_header_value";

-- Analytics Queries

-- name: GetProjectsWithEventsInInterval :many
SELECT p.id, COUNT(e.id) AS events_count
FROM convoy.projects p
LEFT JOIN convoy.events e ON p.id = e.project_id
WHERE e.created_at >= NOW() - MAKE_INTERVAL(hours := $1)
    AND p.deleted_at IS NULL
GROUP BY p.id
ORDER BY events_count DESC;

-- name: CountProjects :one
SELECT COUNT(*) FROM convoy.projects WHERE deleted_at IS NULL;
