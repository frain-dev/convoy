-- Sources Queries

-- ============================================================================
-- CREATE Operations
-- ============================================================================

-- name: CreateSourceVerifier :exec
INSERT INTO convoy.source_verifiers (
    id,
    type,
    basic_username,
    basic_password,
    api_key_header_name,
    api_key_header_value,
    hmac_hash,
    hmac_header,
    hmac_secret,
    hmac_encoding
)
VALUES (
    @id,
    @type,
    @basic_username,
    @basic_password,
    @api_key_header_name,
    @api_key_header_value,
    @hmac_hash,
    @hmac_header,
    @hmac_secret,
    @hmac_encoding
);

-- name: CreateSource :exec
INSERT INTO convoy.sources (
    id,
    source_verifier_id,
    name,
    type,
    mask_id,
    provider,
    is_disabled,
    forward_headers,
    project_id,
    pub_sub,
    custom_response_body,
    custom_response_content_type,
    idempotency_keys,
    body_function,
    header_function
)
VALUES (
    @id,
    @source_verifier_id,
    @name,
    @type,
    @mask_id,
    @provider,
    @is_disabled,
    @forward_headers,
    @project_id,
    @pub_sub,
    @custom_response_body,
    @custom_response_content_type,
    @idempotency_keys,
    @body_function,
    @header_function
);

-- ============================================================================
-- UPDATE Operations
-- ============================================================================

-- name: UpdateSourceVerifier :execresult
UPDATE convoy.source_verifiers
SET
    type = @type,
    basic_username = @basic_username,
    basic_password = @basic_password,
    api_key_header_name = @api_key_header_name,
    api_key_header_value = @api_key_header_value,
    hmac_hash = @hmac_hash,
    hmac_header = @hmac_header,
    hmac_secret = @hmac_secret,
    hmac_encoding = @hmac_encoding,
    updated_at = NOW()
WHERE id = @id AND deleted_at IS NULL;

-- name: UpdateSource :execresult
UPDATE convoy.sources
SET
    name = @name,
    type = @type,
    mask_id = @mask_id,
    provider = @provider,
    is_disabled = @is_disabled,
    forward_headers = @forward_headers,
    project_id = @project_id,
    pub_sub = @pub_sub,
    custom_response_body = @custom_response_body,
    custom_response_content_type = @custom_response_content_type,
    idempotency_keys = @idempotency_keys,
    body_function = @body_function,
    header_function = @header_function,
    updated_at = NOW()
WHERE id = @id AND deleted_at IS NULL;

-- ============================================================================
-- FETCH Operations
-- ============================================================================

-- name: FetchSourceByID :one
SELECT
    s.id,
    s.name,
    s.type,
    s.pub_sub,
    s.mask_id,
    s.provider,
    s.is_disabled,
    s.forward_headers,
    s.idempotency_keys,
    s.project_id,
    s.body_function,
    s.header_function,
    COALESCE(s.source_verifier_id, '') AS source_verifier_id,
    COALESCE(s.custom_response_body, '') AS custom_response_body,
    COALESCE(s.custom_response_content_type, '') AS custom_response_content_type,
    COALESCE(sv.type, '') AS verifier_type,
    COALESCE(sv.basic_username, '') AS verifier_basic_username,
    COALESCE(sv.basic_password, '') AS verifier_basic_password,
    COALESCE(sv.api_key_header_name, '') AS verifier_api_key_header_name,
    COALESCE(sv.api_key_header_value, '') AS verifier_api_key_header_value,
    COALESCE(sv.hmac_hash, '') AS verifier_hmac_hash,
    COALESCE(sv.hmac_header, '') AS verifier_hmac_header,
    COALESCE(sv.hmac_secret, '') AS verifier_hmac_secret,
    COALESCE(sv.hmac_encoding, '') AS verifier_hmac_encoding,
    s.created_at,
    s.updated_at
FROM convoy.sources AS s
LEFT JOIN convoy.source_verifiers sv ON s.source_verifier_id = sv.id
WHERE s.id = @id AND s.deleted_at IS NULL;

-- name: FetchSourceByName :one
SELECT
    s.id,
    s.name,
    s.type,
    s.pub_sub,
    s.mask_id,
    s.provider,
    s.is_disabled,
    s.forward_headers,
    s.idempotency_keys,
    s.project_id,
    s.body_function,
    s.header_function,
    COALESCE(s.source_verifier_id, '') AS source_verifier_id,
    COALESCE(s.custom_response_body, '') AS custom_response_body,
    COALESCE(s.custom_response_content_type, '') AS custom_response_content_type,
    COALESCE(sv.type, '') AS verifier_type,
    COALESCE(sv.basic_username, '') AS verifier_basic_username,
    COALESCE(sv.basic_password, '') AS verifier_basic_password,
    COALESCE(sv.api_key_header_name, '') AS verifier_api_key_header_name,
    COALESCE(sv.api_key_header_value, '') AS verifier_api_key_header_value,
    COALESCE(sv.hmac_hash, '') AS verifier_hmac_hash,
    COALESCE(sv.hmac_header, '') AS verifier_hmac_header,
    COALESCE(sv.hmac_secret, '') AS verifier_hmac_secret,
    COALESCE(sv.hmac_encoding, '') AS verifier_hmac_encoding,
    s.created_at,
    s.updated_at
FROM convoy.sources AS s
LEFT JOIN convoy.source_verifiers sv ON s.source_verifier_id = sv.id
WHERE s.project_id = @project_id AND s.name = @name AND s.deleted_at IS NULL;

-- name: FetchSourceByMaskID :one
SELECT
    s.id,
    s.name,
    s.type,
    s.pub_sub,
    s.mask_id,
    s.provider,
    s.is_disabled,
    s.forward_headers,
    s.idempotency_keys,
    s.project_id,
    s.body_function,
    s.header_function,
    COALESCE(s.source_verifier_id, '') AS source_verifier_id,
    COALESCE(s.custom_response_body, '') AS custom_response_body,
    COALESCE(s.custom_response_content_type, '') AS custom_response_content_type,
    COALESCE(sv.type, '') AS verifier_type,
    COALESCE(sv.basic_username, '') AS verifier_basic_username,
    COALESCE(sv.basic_password, '') AS verifier_basic_password,
    COALESCE(sv.api_key_header_name, '') AS verifier_api_key_header_name,
    COALESCE(sv.api_key_header_value, '') AS verifier_api_key_header_value,
    COALESCE(sv.hmac_hash, '') AS verifier_hmac_hash,
    COALESCE(sv.hmac_header, '') AS verifier_hmac_header,
    COALESCE(sv.hmac_secret, '') AS verifier_hmac_secret,
    COALESCE(sv.hmac_encoding, '') AS verifier_hmac_encoding,
    s.created_at,
    s.updated_at
FROM convoy.sources AS s
LEFT JOIN convoy.source_verifiers sv ON s.source_verifier_id = sv.id
WHERE s.mask_id = @mask_id AND s.deleted_at IS NULL;

-- ============================================================================
-- PAGINATED LIST Operations
-- ============================================================================

-- name: FetchSourcesPaginated :many
-- @direction: 'next' for forward pagination, 'prev' for backward pagination
-- @has_type_filter: true to filter by type, false to skip
-- @has_provider_filter: true to filter by provider, false to skip
-- @has_query_filter: true to filter by name search, false to skip
WITH filtered_sources AS (
    SELECT
        s.id,
        s.name,
        s.type,
        s.pub_sub,
        s.mask_id,
        s.provider,
        s.is_disabled,
        s.forward_headers,
        s.idempotency_keys,
        s.project_id,
        s.body_function,
        s.header_function,
        COALESCE(s.source_verifier_id, '') AS source_verifier_id,
        COALESCE(s.custom_response_body, '') AS custom_response_body,
        COALESCE(s.custom_response_content_type, '') AS custom_response_content_type,
        COALESCE(sv.type, '') AS verifier_type,
        COALESCE(sv.basic_username, '') AS verifier_basic_username,
        COALESCE(sv.basic_password, '') AS verifier_basic_password,
        COALESCE(sv.api_key_header_name, '') AS verifier_api_key_header_name,
        COALESCE(sv.api_key_header_value, '') AS verifier_api_key_header_value,
        COALESCE(sv.hmac_hash, '') AS verifier_hmac_hash,
        COALESCE(sv.hmac_header, '') AS verifier_hmac_header,
        COALESCE(sv.hmac_secret, '') AS verifier_hmac_secret,
        COALESCE(sv.hmac_encoding, '') AS verifier_hmac_encoding,
        s.created_at,
        s.updated_at
    FROM convoy.sources s
    LEFT JOIN convoy.source_verifiers sv ON s.source_verifier_id = sv.id
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
        -- Optional type filter
        AND (
            CASE
                WHEN @has_type_filter::boolean THEN s.type = @type_filter
                ELSE true
            END
        )
        -- Optional provider filter
        AND (
            CASE
                WHEN @has_provider_filter::boolean THEN s.provider = @provider_filter
                ELSE true
            END
        )
        -- Optional name search filter
        AND (
            CASE
                WHEN @has_query_filter::boolean THEN s.name ILIKE @query_filter
                ELSE true
            END
        )
    GROUP BY s.id, sv.id
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
SELECT * FROM filtered_sources
ORDER BY
    CASE
        WHEN @direction::text = 'prev' THEN id
    END DESC,
    CASE
        WHEN @direction::text = 'next' THEN id
    END DESC;

-- name: CountPrevSources :one
-- Unified count query for pagination prev row count
SELECT COUNT(DISTINCT s.id) AS count
FROM convoy.sources s
WHERE s.deleted_at IS NULL
    AND s.project_id = @project_id
    AND s.id > @cursor
    -- Optional type filter
    AND (
        CASE
            WHEN @has_type_filter::boolean THEN s.type = @type_filter
            ELSE true
        END
    )
    -- Optional provider filter
    AND (
        CASE
            WHEN @has_provider_filter::boolean THEN s.provider = @provider_filter
            ELSE true
        END
    )
    -- Optional name search filter
    AND (
        CASE
            WHEN @has_query_filter::boolean THEN s.name ILIKE @query_filter
            ELSE true
        END
    )
GROUP BY s.id
ORDER BY s.id DESC
LIMIT 1;

-- name: FetchPubSubSourcesByProjectIDs :many
-- Fetch PubSub-type sources across multiple projects with pagination
SELECT
    s.id,
    s.name,
    s.type,
    s.pub_sub,
    s.mask_id,
    s.provider,
    s.is_disabled,
    s.forward_headers,
    s.idempotency_keys,
    s.body_function,
    s.header_function,
    s.project_id,
    s.created_at,
    s.updated_at
FROM convoy.sources s
WHERE s.type = @source_type
    AND s.project_id = ANY(@project_ids::text[])
    AND s.deleted_at IS NULL
    AND (s.id <= @cursor OR @cursor = '')
ORDER BY s.id DESC
LIMIT @limit_val;

-- ============================================================================
-- DELETE Operations
-- ============================================================================

-- name: DeleteSource :execresult
UPDATE convoy.sources
SET deleted_at = NOW()
WHERE id = @id AND project_id = @project_id AND deleted_at IS NULL;

-- name: DeleteSourceVerifier :exec
UPDATE convoy.source_verifiers
SET deleted_at = NOW()
WHERE id = @id AND deleted_at IS NULL;

-- name: DeleteSourceSubscriptions :exec
UPDATE convoy.subscriptions
SET deleted_at = NOW()
WHERE source_id = @source_id AND project_id = @project_id AND deleted_at IS NULL;
