-- Subscriptions Queries

-- ============================================================================
-- CREATE Operations
-- ============================================================================

-- name: CreateSubscription :exec
INSERT INTO convoy.subscriptions (
    id,
    name,
    type,
    project_id,
    endpoint_id,
    device_id,
    source_id,
    alert_config_count,
    alert_config_threshold,
    retry_config_type,
    retry_config_duration,
    retry_config_retry_count,
    filter_config_event_types,
    filter_config_filter_headers,
    filter_config_filter_body,
    filter_config_filter_is_flattened,
    filter_config_filter_raw_headers,
    filter_config_filter_raw_body,
    rate_limit_config_count,
    rate_limit_config_duration,
    function,
    delivery_mode
)
VALUES (
    @id,
    @name,
    @type,
    @project_id,
    @endpoint_id,
    @device_id,
    @source_id,
    @alert_config_count,
    @alert_config_threshold,
    @retry_config_type,
    @retry_config_duration,
    @retry_config_retry_count,
    @filter_config_event_types,
    @filter_config_filter_headers,
    @filter_config_filter_body,
    @filter_config_filter_is_flattened,
    @filter_config_filter_raw_headers,
    @filter_config_filter_raw_body,
    @rate_limit_config_count,
    @rate_limit_config_duration,
    @function,
    CASE
        WHEN @delivery_mode = '' OR @delivery_mode IS NULL THEN 'at_least_once'::convoy.delivery_mode
        ELSE @delivery_mode::convoy.delivery_mode
    END
);

-- name: UpsertSubscriptionEventTypes :exec
INSERT INTO convoy.event_types (id, name, project_id, description, category)
VALUES (@id, @name, @project_id, @description, @category)
ON CONFLICT DO NOTHING;

-- name: InsertSubscriptionEventTypeFilters :exec
INSERT INTO convoy.filters (
    id,
    subscription_id,
    event_type,
    headers,
    body,
    raw_headers,
    raw_body
)
SELECT
    convoy.generate_ulid()::VARCHAR,
    id,
    unnest(filter_config_event_types),
    filter_config_filter_headers,
    filter_config_filter_body,
    filter_config_filter_raw_headers,
    filter_config_filter_raw_body
FROM convoy.subscriptions
WHERE id = @subscription_id AND deleted_at IS NULL
ON CONFLICT DO NOTHING;

-- ============================================================================
-- UPDATE Operations
-- ============================================================================

-- name: UpdateSubscription :execresult
UPDATE convoy.subscriptions
SET
    name = @name,
    endpoint_id = @endpoint_id,
    source_id = @source_id,
    alert_config_count = @alert_config_count,
    alert_config_threshold = @alert_config_threshold,
    retry_config_type = @retry_config_type,
    retry_config_duration = @retry_config_duration,
    retry_config_retry_count = @retry_config_retry_count,
    filter_config_event_types = @filter_config_event_types,
    filter_config_filter_headers = @filter_config_filter_headers,
    filter_config_filter_body = @filter_config_filter_body,
    filter_config_filter_is_flattened = @filter_config_filter_is_flattened,
    filter_config_filter_raw_headers = @filter_config_filter_raw_headers,
    filter_config_filter_raw_body = @filter_config_filter_raw_body,
    rate_limit_config_count = @rate_limit_config_count,
    rate_limit_config_duration = @rate_limit_config_duration,
    function = @function,
    delivery_mode = CASE
        WHEN @delivery_mode = '' OR @delivery_mode IS NULL THEN 'at_least_once'::convoy.delivery_mode
        ELSE @delivery_mode::convoy.delivery_mode
    END,
    updated_at = NOW()
WHERE id = @id AND project_id = @project_id AND deleted_at IS NULL;

-- name: DeleteSubscriptionEventTypes :exec
DELETE FROM convoy.filters
WHERE id IN (
    SELECT f.id AS filter_id
    FROM convoy.filters f
    JOIN convoy.subscriptions s ON s.id = f.subscription_id
    WHERE s.id = @subscription_id
        AND f.event_type <> ALL(s.filter_config_event_types)
);

-- ============================================================================
-- FETCH Operations
-- ============================================================================

-- name: FetchSubscriptionByID :one
SELECT
    s.id,
    s.name,
    s.type,
    s.project_id,
    s.created_at,
    s.updated_at,
    s.function,
    s.delivery_mode,
    COALESCE(s.endpoint_id, '') AS endpoint_id,
    COALESCE(s.device_id, '') AS device_id,
    COALESCE(s.source_id, '') AS source_id,
    s.alert_config_count,
    s.alert_config_threshold,
    s.retry_config_type,
    s.retry_config_duration,
    s.retry_config_retry_count,
    s.filter_config_event_types,
    s.filter_config_filter_raw_headers,
    s.filter_config_filter_raw_body,
    s.filter_config_filter_is_flattened,
    s.filter_config_filter_headers,
    s.filter_config_filter_body,
    s.rate_limit_config_count,
    s.rate_limit_config_duration,
    COALESCE(em.id, '') AS endpoint_metadata_id,
    COALESCE(em.name, '') AS endpoint_metadata_name,
    COALESCE(em.project_id, '') AS endpoint_metadata_project_id,
    COALESCE(em.support_email, '') AS endpoint_metadata_support_email,
    COALESCE(em.url, '') AS endpoint_metadata_url,
    COALESCE(em.status, '') AS endpoint_metadata_status,
    COALESCE(em.owner_id, '') AS endpoint_metadata_owner_id,
    COALESCE(em.secrets, '[]'::jsonb) AS endpoint_metadata_secrets,
    COALESCE(d.id, '') AS device_metadata_id,
    COALESCE(d.status, '') AS device_metadata_status,
    COALESCE(d.host_name, '') AS device_metadata_host_name,
    COALESCE(sm.id, '') AS source_metadata_id,
    COALESCE(sm.name, '') AS source_metadata_name,
    COALESCE(sm.type, '') AS source_metadata_type,
    COALESCE(sm.mask_id, '') AS source_metadata_mask_id,
    COALESCE(sm.project_id, '') AS source_metadata_project_id,
    COALESCE(sm.is_disabled, FALSE) AS source_metadata_is_disabled,
    COALESCE(sv.type, '') AS source_verifier_type,
    COALESCE(sv.basic_username, '') AS source_verifier_basic_username,
    COALESCE(sv.basic_password, '') AS source_verifier_basic_password,
    COALESCE(sv.api_key_header_name, '') AS source_verifier_api_key_header_name,
    COALESCE(sv.api_key_header_value, '') AS source_verifier_api_key_header_value,
    COALESCE(sv.hmac_hash, '') AS source_verifier_hmac_hash,
    COALESCE(sv.hmac_header, '') AS source_verifier_hmac_header,
    COALESCE(sv.hmac_secret, '') AS source_verifier_hmac_secret,
    COALESCE(sv.hmac_encoding, '') AS source_verifier_hmac_encoding
FROM convoy.subscriptions s
LEFT JOIN convoy.endpoints em ON s.endpoint_id = em.id
LEFT JOIN convoy.sources sm ON s.source_id = sm.id
LEFT JOIN convoy.source_verifiers sv ON sv.id = sm.source_verifier_id
LEFT JOIN convoy.devices d ON s.device_id = d.id
WHERE s.id = @id AND s.project_id = @project_id AND s.deleted_at IS NULL;

-- name: FetchSubscriptionsBySourceID :many
SELECT
    s.id,
    s.name,
    s.type,
    s.project_id,
    s.created_at,
    s.updated_at,
    s.function,
    s.delivery_mode,
    COALESCE(s.endpoint_id, '') AS endpoint_id,
    COALESCE(s.device_id, '') AS device_id,
    COALESCE(s.source_id, '') AS source_id,
    s.alert_config_count,
    s.alert_config_threshold,
    s.retry_config_type,
    s.retry_config_duration,
    s.retry_config_retry_count,
    s.filter_config_event_types,
    s.filter_config_filter_raw_headers,
    s.filter_config_filter_raw_body,
    s.filter_config_filter_is_flattened,
    s.filter_config_filter_headers,
    s.filter_config_filter_body,
    s.rate_limit_config_count,
    s.rate_limit_config_duration,
    COALESCE(em.id, '') AS endpoint_metadata_id,
    COALESCE(em.name, '') AS endpoint_metadata_name,
    COALESCE(em.project_id, '') AS endpoint_metadata_project_id,
    COALESCE(em.support_email, '') AS endpoint_metadata_support_email,
    COALESCE(em.url, '') AS endpoint_metadata_url,
    COALESCE(em.status, '') AS endpoint_metadata_status,
    COALESCE(em.owner_id, '') AS endpoint_metadata_owner_id,
    COALESCE(em.secrets, '[]'::jsonb) AS endpoint_metadata_secrets,
    COALESCE(d.id, '') AS device_metadata_id,
    COALESCE(d.status, '') AS device_metadata_status,
    COALESCE(d.host_name, '') AS device_metadata_host_name,
    COALESCE(sm.id, '') AS source_metadata_id,
    COALESCE(sm.name, '') AS source_metadata_name,
    COALESCE(sm.type, '') AS source_metadata_type,
    COALESCE(sm.mask_id, '') AS source_metadata_mask_id,
    COALESCE(sm.project_id, '') AS source_metadata_project_id,
    COALESCE(sm.is_disabled, FALSE) AS source_metadata_is_disabled,
    COALESCE(sv.type, '') AS source_verifier_type,
    COALESCE(sv.basic_username, '') AS source_verifier_basic_username,
    COALESCE(sv.basic_password, '') AS source_verifier_basic_password,
    COALESCE(sv.api_key_header_name, '') AS source_verifier_api_key_header_name,
    COALESCE(sv.api_key_header_value, '') AS source_verifier_api_key_header_value,
    COALESCE(sv.hmac_hash, '') AS source_verifier_hmac_hash,
    COALESCE(sv.hmac_header, '') AS source_verifier_hmac_header,
    COALESCE(sv.hmac_secret, '') AS source_verifier_hmac_secret,
    COALESCE(sv.hmac_encoding, '') AS source_verifier_hmac_encoding
FROM convoy.subscriptions s
LEFT JOIN convoy.endpoints em ON s.endpoint_id = em.id
LEFT JOIN convoy.sources sm ON s.source_id = sm.id
LEFT JOIN convoy.source_verifiers sv ON sv.id = sm.source_verifier_id
LEFT JOIN convoy.devices d ON s.device_id = d.id
WHERE s.project_id = @project_id AND s.source_id = @source_id AND s.deleted_at IS NULL;

-- name: FetchSubscriptionsByEndpointID :many
SELECT
    s.id,
    s.name,
    s.type,
    s.project_id,
    s.created_at,
    s.updated_at,
    s.function,
    s.delivery_mode,
    COALESCE(s.endpoint_id, '') AS endpoint_id,
    COALESCE(s.device_id, '') AS device_id,
    COALESCE(s.source_id, '') AS source_id,
    s.alert_config_count,
    s.alert_config_threshold,
    s.retry_config_type,
    s.retry_config_duration,
    s.retry_config_retry_count,
    s.filter_config_event_types,
    s.filter_config_filter_raw_headers,
    s.filter_config_filter_raw_body,
    s.filter_config_filter_is_flattened,
    s.filter_config_filter_headers,
    s.filter_config_filter_body,
    s.rate_limit_config_count,
    s.rate_limit_config_duration,
    COALESCE(em.id, '') AS endpoint_metadata_id,
    COALESCE(em.name, '') AS endpoint_metadata_name,
    COALESCE(em.project_id, '') AS endpoint_metadata_project_id,
    COALESCE(em.support_email, '') AS endpoint_metadata_support_email,
    COALESCE(em.url, '') AS endpoint_metadata_url,
    COALESCE(em.status, '') AS endpoint_metadata_status,
    COALESCE(em.owner_id, '') AS endpoint_metadata_owner_id,
    COALESCE(em.secrets, '[]'::jsonb) AS endpoint_metadata_secrets,
    COALESCE(d.id, '') AS device_metadata_id,
    COALESCE(d.status, '') AS device_metadata_status,
    COALESCE(d.host_name, '') AS device_metadata_host_name,
    COALESCE(sm.id, '') AS source_metadata_id,
    COALESCE(sm.name, '') AS source_metadata_name,
    COALESCE(sm.type, '') AS source_metadata_type,
    COALESCE(sm.mask_id, '') AS source_metadata_mask_id,
    COALESCE(sm.project_id, '') AS source_metadata_project_id,
    COALESCE(sm.is_disabled, FALSE) AS source_metadata_is_disabled,
    COALESCE(sv.type, '') AS source_verifier_type,
    COALESCE(sv.basic_username, '') AS source_verifier_basic_username,
    COALESCE(sv.basic_password, '') AS source_verifier_basic_password,
    COALESCE(sv.api_key_header_name, '') AS source_verifier_api_key_header_name,
    COALESCE(sv.api_key_header_value, '') AS source_verifier_api_key_header_value,
    COALESCE(sv.hmac_hash, '') AS source_verifier_hmac_hash,
    COALESCE(sv.hmac_header, '') AS source_verifier_hmac_header,
    COALESCE(sv.hmac_secret, '') AS source_verifier_hmac_secret,
    COALESCE(sv.hmac_encoding, '') AS source_verifier_hmac_encoding
FROM convoy.subscriptions s
LEFT JOIN convoy.endpoints em ON s.endpoint_id = em.id
LEFT JOIN convoy.sources sm ON s.source_id = sm.id
LEFT JOIN convoy.source_verifiers sv ON sv.id = sm.source_verifier_id
LEFT JOIN convoy.devices d ON s.device_id = d.id
WHERE s.project_id = @project_id AND s.endpoint_id = @endpoint_id AND s.deleted_at IS NULL;

-- name: FetchSubscriptionByDeviceID :one
SELECT
    s.id,
    s.name,
    s.type,
    s.project_id,
    s.created_at,
    s.updated_at,
    s.function,
    s.delivery_mode,
    COALESCE(s.endpoint_id, '') AS endpoint_id,
    COALESCE(s.device_id, '') AS device_id,
    COALESCE(s.source_id, '') AS source_id,
    s.alert_config_count,
    s.alert_config_threshold,
    s.retry_config_type,
    s.retry_config_duration,
    s.retry_config_retry_count,
    s.filter_config_event_types,
    s.filter_config_filter_raw_headers,
    s.filter_config_filter_raw_body,
    s.filter_config_filter_is_flattened,
    s.filter_config_filter_headers,
    s.filter_config_filter_body,
    s.rate_limit_config_count,
    s.rate_limit_config_duration,
    COALESCE(d.id, '') AS device_metadata_id,
    COALESCE(d.status, '') AS device_metadata_status,
    COALESCE(d.host_name, '') AS device_metadata_host_name
FROM convoy.subscriptions s
LEFT JOIN convoy.devices d ON s.device_id = d.id
WHERE s.device_id = @device_id AND s.project_id = @project_id AND s.type = @subscription_type AND s.deleted_at IS NULL;

-- name: FetchCLISubscriptions :many
SELECT
    s.id,
    s.name,
    s.type,
    s.project_id,
    s.created_at,
    s.updated_at,
    s.function,
    s.delivery_mode,
    COALESCE(s.endpoint_id, '') AS endpoint_id,
    COALESCE(s.device_id, '') AS device_id,
    COALESCE(s.source_id, '') AS source_id,
    s.alert_config_count,
    s.alert_config_threshold,
    s.retry_config_type,
    s.retry_config_duration,
    s.retry_config_retry_count,
    s.filter_config_event_types,
    s.filter_config_filter_raw_headers,
    s.filter_config_filter_raw_body,
    s.filter_config_filter_is_flattened,
    s.filter_config_filter_headers,
    s.filter_config_filter_body,
    s.rate_limit_config_count,
    s.rate_limit_config_duration,
    COALESCE(em.id, '') AS endpoint_metadata_id,
    COALESCE(em.name, '') AS endpoint_metadata_name,
    COALESCE(em.project_id, '') AS endpoint_metadata_project_id,
    COALESCE(em.support_email, '') AS endpoint_metadata_support_email,
    COALESCE(em.url, '') AS endpoint_metadata_url,
    COALESCE(em.status, '') AS endpoint_metadata_status,
    COALESCE(em.owner_id, '') AS endpoint_metadata_owner_id,
    COALESCE(em.secrets, '[]'::jsonb) AS endpoint_metadata_secrets,
    COALESCE(d.id, '') AS device_metadata_id,
    COALESCE(d.status, '') AS device_metadata_status,
    COALESCE(d.host_name, '') AS device_metadata_host_name,
    COALESCE(sm.id, '') AS source_metadata_id,
    COALESCE(sm.name, '') AS source_metadata_name,
    COALESCE(sm.type, '') AS source_metadata_type,
    COALESCE(sm.mask_id, '') AS source_metadata_mask_id,
    COALESCE(sm.project_id, '') AS source_metadata_project_id,
    COALESCE(sm.is_disabled, FALSE) AS source_metadata_is_disabled,
    COALESCE(sv.type, '') AS source_verifier_type,
    COALESCE(sv.basic_username, '') AS source_verifier_basic_username,
    COALESCE(sv.basic_password, '') AS source_verifier_basic_password,
    COALESCE(sv.api_key_header_name, '') AS source_verifier_api_key_header_name,
    COALESCE(sv.api_key_header_value, '') AS source_verifier_api_key_header_value,
    COALESCE(sv.hmac_hash, '') AS source_verifier_hmac_hash,
    COALESCE(sv.hmac_header, '') AS source_verifier_hmac_header,
    COALESCE(sv.hmac_secret, '') AS source_verifier_hmac_secret,
    COALESCE(sv.hmac_encoding, '') AS source_verifier_hmac_encoding
FROM convoy.subscriptions s
LEFT JOIN convoy.endpoints em ON s.endpoint_id = em.id
LEFT JOIN convoy.sources sm ON s.source_id = sm.id
LEFT JOIN convoy.source_verifiers sv ON sv.id = sm.source_verifier_id
LEFT JOIN convoy.devices d ON s.device_id = d.id
WHERE s.project_id = @project_id AND s.type = 'cli' AND s.deleted_at IS NULL;

-- ============================================================================
-- PAGINATED Operations
-- ============================================================================

-- name: FetchSubscriptionsPaginated :many
-- @direction: 'next' for forward pagination, 'prev' for backward pagination
-- @has_endpoint_filter: true to filter by endpoint_ids, false to skip
-- @has_name_filter: true to filter by name, false to skip
WITH filtered_subscriptions AS (
    SELECT
        s.id,
        s.name,
        s.type,
        s.project_id,
        s.created_at,
        s.updated_at,
        s.function,
        s.delivery_mode,
        COALESCE(s.endpoint_id, '') AS endpoint_id,
        COALESCE(s.device_id, '') AS device_id,
        COALESCE(s.source_id, '') AS source_id,
        s.alert_config_count,
        s.alert_config_threshold,
        s.retry_config_type,
        s.retry_config_duration,
        s.retry_config_retry_count,
        s.filter_config_event_types,
        s.filter_config_filter_raw_headers,
        s.filter_config_filter_raw_body,
        s.filter_config_filter_is_flattened,
        s.filter_config_filter_headers,
        s.filter_config_filter_body,
        s.rate_limit_config_count,
        s.rate_limit_config_duration,
        COALESCE(em.id, '') AS endpoint_metadata_id,
        COALESCE(em.name, '') AS endpoint_metadata_name,
        COALESCE(em.project_id, '') AS endpoint_metadata_project_id,
        COALESCE(em.support_email, '') AS endpoint_metadata_support_email,
        COALESCE(em.url, '') AS endpoint_metadata_url,
        COALESCE(em.status, '') AS endpoint_metadata_status,
        COALESCE(em.owner_id, '') AS endpoint_metadata_owner_id,
        COALESCE(em.secrets, '[]'::jsonb) AS endpoint_metadata_secrets,
        COALESCE(d.id, '') AS device_metadata_id,
        COALESCE(d.status, '') AS device_metadata_status,
        COALESCE(d.host_name, '') AS device_metadata_host_name,
        COALESCE(sm.id, '') AS source_metadata_id,
        COALESCE(sm.name, '') AS source_metadata_name,
        COALESCE(sm.type, '') AS source_metadata_type,
        COALESCE(sm.mask_id, '') AS source_metadata_mask_id,
        COALESCE(sm.project_id, '') AS source_metadata_project_id,
        COALESCE(sm.is_disabled, FALSE) AS source_metadata_is_disabled,
        COALESCE(sv.type, '') AS source_verifier_type,
        COALESCE(sv.basic_username, '') AS source_verifier_basic_username,
        COALESCE(sv.basic_password, '') AS source_verifier_basic_password,
        COALESCE(sv.api_key_header_name, '') AS source_verifier_api_key_header_name,
        COALESCE(sv.api_key_header_value, '') AS source_verifier_api_key_header_value,
        COALESCE(sv.hmac_hash, '') AS source_verifier_hmac_hash,
        COALESCE(sv.hmac_header, '') AS source_verifier_hmac_header,
        COALESCE(sv.hmac_secret, '') AS source_verifier_hmac_secret,
        COALESCE(sv.hmac_encoding, '') AS source_verifier_hmac_encoding
    FROM convoy.subscriptions s
    LEFT JOIN convoy.endpoints em ON s.endpoint_id = em.id
    LEFT JOIN convoy.sources sm ON s.source_id = sm.id
    LEFT JOIN convoy.source_verifiers sv ON sv.id = sm.source_verifier_id
    LEFT JOIN convoy.devices d ON s.device_id = d.id
    WHERE s.deleted_at IS NULL
        AND s.project_id = @project_id
        -- Cursor comparison: <= for forward (next), >= for backward (prev)
        AND (
            CASE
                WHEN @direction::text = 'next' THEN s.id <= @cursor
                WHEN @direction::text = 'prev' THEN s.id >= @cursor
                ELSE true
            END
        )
        -- Optional endpoint filter
        AND (
            CASE
                WHEN @has_endpoint_filter::boolean THEN s.endpoint_id = ANY(@endpoint_ids::text[])
                ELSE true
            END
        )
        -- Optional name search filter
        AND (
            CASE
                WHEN @has_name_filter::boolean THEN s.name ILIKE @name_filter
                ELSE true
            END
        )
    GROUP BY s.id, em.id, sm.id, sv.id, d.id
    -- Sort order: DESC for forward, ASC for backward (will be reversed in outer query for backward)
    ORDER BY
        CASE
            WHEN @direction::text = 'next' THEN s.id
        END DESC,
        CASE
            WHEN @direction::text = 'prev' THEN s.id
        END ASC
    LIMIT @limit_val
)
-- Final select: reverse order for backward pagination to get DESC order
SELECT * FROM filtered_subscriptions
ORDER BY
    CASE
        WHEN @direction::text = 'prev' THEN id
    END DESC,
    CASE
        WHEN @direction::text = 'next' THEN id
    END DESC;

-- name: CountPrevSubscriptions :one
-- Unified count query for pagination prev row count
SELECT COUNT(DISTINCT s.id) AS count
FROM convoy.subscriptions s
WHERE s.deleted_at IS NULL
    AND s.project_id = @project_id
    AND s.id > @cursor
    -- Optional endpoint filter
    AND (
        CASE
            WHEN @has_endpoint_filter::boolean THEN s.endpoint_id = ANY(@endpoint_ids::text[])
            ELSE true
        END
    )
    -- Optional name search filter
    AND (
        CASE
            WHEN @has_name_filter::boolean THEN s.name ILIKE @name_filter
            ELSE true
        END
    );

-- ============================================================================
-- BROADCAST & SYNC Operations
-- ============================================================================

-- name: FetchSubscriptionsForBroadcast :many
-- Fetch subscriptions matching event type with pagination
SELECT
    id,
    type,
    project_id,
    endpoint_id,
    function,
    filter_config_event_types,
    filter_config_filter_headers,
    filter_config_filter_body,
    filter_config_filter_is_flattened
FROM convoy.subscriptions
WHERE (ARRAY[@event_type] <@ filter_config_event_types OR ARRAY['*'] <@ filter_config_event_types)
    AND id > @cursor
    AND project_id = @project_id
    AND deleted_at IS NULL
ORDER BY id
LIMIT @limit_val;

-- name: LoadAllSubscriptionsConfiguration :many
-- Load all subscription configs for multiple projects with pagination
SELECT
    name,
    id,
    type,
    project_id,
    endpoint_id,
    function,
    updated_at,
    filter_config_event_types,
    filter_config_filter_headers,
    filter_config_filter_body,
    filter_config_filter_is_flattened
FROM convoy.subscriptions
WHERE id > @cursor
    AND project_id = ANY(@project_ids::text[])
    AND deleted_at IS NULL
ORDER BY id
LIMIT @limit_val;

-- name: FetchUpdatedSubscriptions :many
-- Fetch subscriptions that have been updated since last sync
-- Uses VALUES clause for input map (id, last_updated_at pairs)
-- @values_clause will be substituted with the actual VALUES
WITH input_map(id, last_updated_at) AS (
    VALUES (@subscription_id_1, @updated_at_1::timestamptz)
),
updated_existing AS (
    SELECT
        s.name,
        s.id,
        s.type,
        s.project_id,
        s.endpoint_id,
        s.function,
        s.updated_at,
        s.filter_config_event_types,
        s.filter_config_filter_headers,
        s.filter_config_filter_body,
        s.filter_config_filter_is_flattened,
        s.filter_config_filter_raw_headers,
        s.filter_config_filter_raw_body
    FROM convoy.subscriptions s
    JOIN input_map m ON s.id = m.id
    WHERE s.updated_at > m.last_updated_at
        AND s.project_id = ANY(@project_ids::text[])
        AND s.deleted_at IS NULL
),
new_subscriptions AS (
    SELECT
        s.name,
        s.id,
        s.type,
        s.project_id,
        s.endpoint_id,
        s.function,
        s.updated_at,
        s.filter_config_event_types,
        s.filter_config_filter_headers,
        s.filter_config_filter_body,
        s.filter_config_filter_is_flattened,
        s.filter_config_filter_raw_headers,
        s.filter_config_filter_raw_body
    FROM convoy.subscriptions s
    WHERE s.id NOT IN (SELECT id FROM input_map)
        AND s.project_id = ANY(@project_ids::text[])
        AND s.deleted_at IS NULL
)
SELECT * FROM updated_existing
UNION ALL
SELECT * FROM new_subscriptions
ORDER BY id
LIMIT @limit_val;

-- name: FetchNewSubscriptions :many
-- Fetch new subscriptions created after last sync time
SELECT
    s.name,
    s.id,
    s.type,
    s.project_id,
    s.endpoint_id,
    s.function,
    s.updated_at,
    s.filter_config_event_types,
    s.filter_config_filter_headers,
    s.filter_config_filter_body,
    s.filter_config_filter_is_flattened,
    s.filter_config_filter_raw_headers,
    s.filter_config_filter_raw_body
FROM convoy.subscriptions s
WHERE s.created_at > @last_sync_time
    AND (@has_known_ids::boolean = false OR s.id <> ALL(@known_subscription_ids::text[]))
    AND s.project_id = ANY(@project_ids::text[])
    AND s.deleted_at IS NULL
ORDER BY s.id
LIMIT @limit_val;

-- name: FetchDeletedSubscriptions :many
-- Fetch subscriptions that have been deleted
SELECT
    id,
    deleted_at,
    project_id,
    filter_config_event_types
FROM convoy.subscriptions
WHERE deleted_at IS NOT NULL
    AND id = ANY(@subscription_ids::text[])
    AND project_id = ANY(@project_ids::text[])
ORDER BY id
LIMIT @limit_val;

-- ============================================================================
-- UTILITY Operations
-- ============================================================================

-- name: CountEndpointSubscriptions :one
-- Count subscriptions for a specific endpoint (excluding a specific subscription)
SELECT COUNT(s.id) AS count
FROM convoy.subscriptions s
WHERE s.deleted_at IS NULL
    AND s.project_id = @project_id
    AND s.endpoint_id = @endpoint_id
    AND s.id <> @exclude_subscription_id;

-- name: CountProjectSubscriptions :one
-- Count all subscriptions across multiple projects
SELECT COUNT(s.id) AS count
FROM convoy.subscriptions s
WHERE s.deleted_at IS NULL
    AND s.project_id = ANY(@project_ids::text[]);

-- ============================================================================
-- DELETE Operations
-- ============================================================================

-- name: DeleteSubscription :execresult
UPDATE convoy.subscriptions
SET deleted_at = NOW()
WHERE id = @id AND project_id = @project_id AND deleted_at IS NULL;
