-- Meta Events Queries
-- Schema: convoy.meta_events
-- Columns: id, project_id, event_type, metadata, attempt, status, created_at, updated_at, deleted_at

-- ============================================================================
-- CREATE Operations
-- ============================================================================

-- name: CreateMetaEvent :exec
INSERT INTO convoy.meta_events (
    id, event_type, project_id, metadata, status
) VALUES (
    @id, @event_type, @project_id, @metadata, @status
);

-- ============================================================================
-- READ Operations - Single Record
-- ============================================================================

-- name: FindMetaEventByID :one
SELECT
    id,
    project_id,
    event_type,
    metadata,
    attempt,
    status,
    created_at,
    updated_at
FROM convoy.meta_events
WHERE id = @id AND project_id = @project_id AND deleted_at IS NULL;

-- ============================================================================
-- UPDATE Operations
-- ============================================================================

-- name: UpdateMetaEvent :execresult
UPDATE convoy.meta_events
SET
    event_type = @event_type,
    metadata = @metadata,
    attempt = @attempt,
    status = @status,
    updated_at = NOW()
WHERE id = @id AND project_id = @project_id AND deleted_at IS NULL;

-- ============================================================================
-- PAGINATED READ Operations
-- ============================================================================

-- name: FetchMetaEventsPaginated :many
-- Unified pagination query that handles:
-- - Bidirectional pagination (forward with 'next', backward with 'prev')
-- - Date range filtering
-- - Soft delete filtering (deleted_at IS NULL)
--
-- Parameters:
-- @direction: 'next' for forward pagination, 'prev' for backward pagination
-- @cursor: ID of the last item from previous page (empty string for first page)
-- @project_id: Filter by project_id
-- @start_date: Start of date range filter
-- @end_date: End of date range filter
-- @limit_val: Number of records to fetch (should be PerPage + 1 for hasNext detection)
WITH filtered_meta_events AS (
    SELECT
        id,
        project_id,
        event_type,
        metadata,
        attempt,
        status,
        created_at,
        updated_at
    FROM convoy.meta_events
    WHERE deleted_at IS NULL
        AND project_id = @project_id
        AND created_at >= @start_date
        AND created_at <= @end_date
        -- Cursor-based pagination: <= for forward (next), >= for backward (prev)
        -- Skip cursor check if cursor is empty (first page)
        AND (
            CASE
                WHEN @cursor = '' THEN true
                WHEN @direction::text = 'next' THEN id <= @cursor
                WHEN @direction::text = 'prev' THEN id >= @cursor
                ELSE true
            END
        )
    ORDER BY
        CASE WHEN @direction::text = 'next' THEN id END DESC,
        CASE WHEN @direction::text = 'prev' THEN id END ASC
    LIMIT @limit_val
)
-- Final select: reverse order for backward pagination to maintain DESC ordering
SELECT * FROM filtered_meta_events
ORDER BY
    CASE WHEN @direction::text = 'prev' THEN id END DESC,
    CASE WHEN @direction::text = 'next' THEN id END DESC;

-- name: CountPrevMetaEvents :one
-- Count records before the given cursor (for pagination metadata)
-- Uses same filters as FetchMetaEventsPaginated
--
-- This query counts how many records exist with ID > cursor
-- which tells us how many records are on previous pages
SELECT COALESCE(COUNT(DISTINCT id), 0) AS count
FROM convoy.meta_events
WHERE deleted_at IS NULL
    AND project_id = @project_id
    AND created_at >= @start_date
    AND created_at <= @end_date
    AND id > @cursor;
