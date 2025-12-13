-- API Keys Queries
-- Schema: convoy.api_keys
-- Columns: id, name, key_type, mask_id, role_type, role_project, role_endpoint,
--          hash, salt, user_id, expires_at, created_at, updated_at, deleted_at

-- ============================================================================
-- CREATE Operations
-- ============================================================================

-- name: CreateAPIKey :exec
INSERT INTO convoy.api_keys (
    id, name, key_type, mask_id,
    role_type, role_project, role_endpoint,
    hash, salt, user_id, expires_at
)
VALUES (
    @id, @name, @key_type, @mask_id,
    @role_type, @role_project, @role_endpoint,
    @hash, @salt, @user_id, @expires_at
);

-- ============================================================================
-- UPDATE Operations
-- ============================================================================

-- name: UpdateAPIKey :exec
UPDATE convoy.api_keys
SET
    name = @name,
    role_type = @role_type,
    role_project = @role_project,
    role_endpoint = @role_endpoint,
    updated_at = NOW()
WHERE id = @id AND deleted_at IS NULL;

-- ============================================================================
-- READ Operations - Single Record
-- ============================================================================

-- name: FindAPIKeyByID :one
SELECT
    id,
    name,
    key_type,
    mask_id,
    COALESCE(role_type, '') AS role_type,
    COALESCE(role_project, '') AS role_project,
    COALESCE(role_endpoint, '') AS role_endpoint,
    hash,
    salt,
    COALESCE(user_id, '') AS user_id,
    created_at,
    updated_at,
    expires_at
FROM convoy.api_keys
WHERE id = @id AND deleted_at IS NULL;

-- name: FindAPIKeyByMaskID :one
-- CRITICAL: Used for API key authentication in NativeRealm
SELECT
    id,
    name,
    key_type,
    mask_id,
    COALESCE(role_type, '') AS role_type,
    COALESCE(role_project, '') AS role_project,
    COALESCE(role_endpoint, '') AS role_endpoint,
    hash,
    salt,
    COALESCE(user_id, '') AS user_id,
    created_at,
    updated_at,
    expires_at
FROM convoy.api_keys
WHERE mask_id = @mask_id AND deleted_at IS NULL;

-- name: FindAPIKeyByHash :one
SELECT
    id,
    name,
    key_type,
    mask_id,
    COALESCE(role_type, '') AS role_type,
    COALESCE(role_project, '') AS role_project,
    COALESCE(role_endpoint, '') AS role_endpoint,
    hash,
    salt,
    COALESCE(user_id, '') AS user_id,
    created_at,
    updated_at,
    expires_at
FROM convoy.api_keys
WHERE hash = @hash AND deleted_at IS NULL;

-- name: FindAPIKeyByProjectID :one
SELECT
    id,
    name,
    key_type,
    mask_id,
    COALESCE(role_type, '') AS role_type,
    COALESCE(role_project, '') AS role_project,
    COALESCE(role_endpoint, '') AS role_endpoint,
    hash,
    salt,
    COALESCE(user_id, '') AS user_id,
    created_at,
    updated_at,
    expires_at
FROM convoy.api_keys
WHERE role_project = @role_project AND deleted_at IS NULL;

-- ============================================================================
-- DELETE Operations (Soft Delete)
-- ============================================================================

-- name: RevokeAPIKeys :exec
-- Soft delete multiple API keys by setting deleted_at timestamp
-- Uses ANY() for array parameter handling
UPDATE convoy.api_keys
SET deleted_at = NOW()
WHERE id = ANY(@ids::text[]) AND deleted_at IS NULL;

-- ============================================================================
-- PAGINATED READ Operations
-- ============================================================================

-- name: FetchAPIKeysPaginated :many
-- Unified pagination query that handles:
-- - Bidirectional pagination (forward with 'next', backward with 'prev')
-- - Multiple optional filters (project_id, endpoint_id, user_id, key_type)
-- - Array filter for endpoint_ids
-- - Soft delete filtering (deleted_at IS NULL)
--
-- Parameters:
-- @direction: 'next' for forward pagination, 'prev' for backward pagination
-- @cursor: ID of the last item from previous page (empty string for first page)
-- @project_id: Filter by role_project (empty string to skip)
-- @endpoint_id: Filter by single role_endpoint (empty string to skip)
-- @user_id: Filter by user_id (empty string to skip)
-- @key_type: Filter by key_type (empty string to skip)
-- @has_endpoint_ids: true to filter by endpoint_ids array, false to skip
-- @endpoint_ids: Array of endpoint IDs to filter by
-- @limit_val: Number of records to fetch (should be PerPage + 1 for hasNext detection)
WITH filtered_api_keys AS (
    SELECT
        id,
        name,
        key_type,
        mask_id,
        COALESCE(role_type, '') AS role_type,
        COALESCE(role_project, '') AS role_project,
        COALESCE(role_endpoint, '') AS role_endpoint,
        hash,
        salt,
        COALESCE(user_id, '') AS user_id,
        created_at,
        updated_at,
        expires_at
    FROM convoy.api_keys
    WHERE deleted_at IS NULL
        -- Cursor-based pagination: < for forward (next), > for backward (prev)
        -- Skip cursor check if cursor is empty (first page)
        AND (
            CASE
                WHEN @cursor = '' THEN true
                WHEN @direction::text = 'next' THEN id < @cursor
                WHEN @direction::text = 'prev' THEN id > @cursor
                ELSE true
            END
        )
        -- Optional filters: apply only if value is not empty string
        AND (@project_id = '' OR role_project = @project_id)
        AND (@endpoint_id = '' OR role_endpoint = @endpoint_id)
        AND (@user_id = '' OR user_id = @user_id)
        AND (@key_type = '' OR key_type = @key_type)
        -- Array filter: apply only if has_endpoint_ids is true
        AND (
            CASE
                WHEN @has_endpoint_ids::boolean THEN role_endpoint = ANY(@endpoint_ids::text[])
                ELSE true
            END
        )
    GROUP BY id
    -- Sort order: DESC for forward (next), ASC for backward (prev)
    -- This ensures we get the right direction for cursor-based pagination
    ORDER BY
        CASE WHEN @direction::text = 'next' THEN id END DESC,
        CASE WHEN @direction::text = 'prev' THEN id END ASC
    LIMIT @limit_val
)
-- Final select: reverse order for backward pagination to maintain DESC ordering
SELECT * FROM filtered_api_keys
ORDER BY
    CASE WHEN @direction::text = 'prev' THEN id END DESC,
    CASE WHEN @direction::text = 'next' THEN id END DESC;

-- name: CountPrevAPIKeys :one
-- Count records before the given cursor (for pagination metadata)
-- Uses same filters as FetchAPIKeysPaginated
--
-- This query counts how many records exist with ID > cursor
-- which tells us how many records are on previous pages
SELECT COALESCE(COUNT(DISTINCT id), 0) AS count
FROM convoy.api_keys
WHERE deleted_at IS NULL
    AND id > @cursor
    -- Apply same filters as pagination query
    AND (@project_id = '' OR role_project = @project_id)
    AND (@endpoint_id = '' OR role_endpoint = @endpoint_id)
    AND (@user_id = '' OR user_id = @user_id)
    AND (@key_type = '' OR key_type = @key_type)
    AND (
        CASE
            WHEN @has_endpoint_ids::boolean THEN role_endpoint = ANY(@endpoint_ids::text[])
            ELSE true
        END
    );