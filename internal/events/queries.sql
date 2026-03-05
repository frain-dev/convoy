-- Events Repository SQL Queries
-- Total: 19 queries organized into 5 groups

-- ============================================================================
-- Group 1: Simple CRUD Operations (5 queries)
-- ============================================================================

-- name: CreateEvent :exec
INSERT INTO convoy.events (
    id, event_type, endpoints, project_id, source_id,
    headers, raw, data, url_query_params, idempotency_key,
    is_duplicate_event, acknowledged_at, metadata, status
)
VALUES (
    @id, @event_type, @endpoints, @project_id, @source_id,
    @headers, @raw, @data, @url_query_params, @idempotency_key,
    @is_duplicate_event, @acknowledged_at, @metadata, @status
);

-- name: CreateEventEndpoints :exec
INSERT INTO convoy.events_endpoints (event_id, endpoint_id)
VALUES (@event_id, @endpoint_id)
ON CONFLICT (endpoint_id, event_id) DO NOTHING;

-- name: UpdateEventEndpoints :exec
UPDATE convoy.events
SET endpoints = @endpoints
WHERE project_id = @project_id AND id = @id;

-- name: UpdateEventStatus :exec
UPDATE convoy.events
SET status = @status
WHERE project_id = @project_id AND id = @id;

-- name: FindEventByID :one
SELECT
    ev.id, ev.event_type, ev.endpoints, ev.project_id, ev.raw, ev.data,
    ev.headers, ev.is_duplicate_event,
    COALESCE(ev.source_id, '') AS source_id,
    COALESCE(ev.idempotency_key, '') AS idempotency_key,
    COALESCE(ev.url_query_params, '') AS url_query_params,
    ev.created_at, ev.updated_at, ev.acknowledged_at, ev.metadata, ev.status,
    COALESCE(s.id, '') AS "source_metadata.id",
    COALESCE(s.name, '') AS "source_metadata.name"
FROM convoy.events ev
LEFT JOIN convoy.sources s ON s.id = ev.source_id
WHERE ev.id = @id AND ev.project_id = @project_id AND ev.deleted_at IS NULL;

-- ============================================================================
-- Group 2: Batch Reads & Counting (5 queries)
-- ============================================================================

-- name: FindEventsByIDs :many
SELECT
    ev.id, ev.project_id, ev.is_duplicate_event, ev.event_type AS event_type,
    COALESCE(ev.source_id, '') AS source_id,
    COALESCE(ev.idempotency_key, '') AS idempotency_key,
    COALESCE(ev.url_query_params, '') AS url_query_params,
    ev.headers, ev.raw, ev.data, ev.created_at, ev.updated_at, ev.deleted_at, ev.acknowledged_at,
    COALESCE(s.id, '') AS "source_metadata.id",
    COALESCE(s.name, '') AS "source_metadata.name"
FROM convoy.events ev
LEFT JOIN convoy.events_endpoints ee ON ee.event_id = ev.id
LEFT JOIN convoy.endpoints e ON e.id = ee.endpoint_id
LEFT JOIN convoy.sources s ON s.id = ev.source_id
WHERE ev.deleted_at IS NULL
    AND ev.id = ANY(@event_ids::text[])
    AND ev.project_id = @project_id;

-- name: FindEventsByIdempotencyKey :many
SELECT id
FROM convoy.events
WHERE idempotency_key = @idempotency_key
    AND project_id = @project_id
    AND deleted_at IS NULL;

-- name: FindFirstEventWithIdempotencyKey :one
SELECT id
FROM convoy.events
WHERE idempotency_key = @idempotency_key
    AND is_duplicate_event IS FALSE
    AND project_id = @project_id
    AND deleted_at IS NULL
ORDER BY created_at
LIMIT 1;

-- name: CountProjectMessages :one
SELECT COUNT(project_id)
FROM convoy.events
WHERE project_id = @project_id AND deleted_at IS NULL;

-- name: CountEvents :one
SELECT COUNT(DISTINCT(ev.id))
FROM convoy.events ev
LEFT JOIN convoy.events_endpoints ee ON ee.event_id = ev.id
LEFT JOIN convoy.endpoints e ON ee.endpoint_id = e.id
WHERE ev.project_id = @project_id
    AND ev.created_at >= @start_date
    AND ev.created_at <= @end_date
    AND ev.deleted_at IS NULL
    AND (CASE WHEN @has_endpoint_ids::boolean THEN e.id = ANY(@endpoint_ids::text[]) ELSE true END)
    AND (CASE WHEN @has_source_id::boolean THEN ev.source_id = @source_id ELSE true END);

-- ============================================================================
-- Group 3: Complex Pagination (5 queries) ⚠️ MOST CRITICAL
-- ============================================================================

-- name: LoadEventsPagedExists :many
-- Fast pagination using EXISTS subquery (no search query)
-- Leverages idx_events_project_created_pagination index
SELECT
    ev.id, ev.project_id, ev.event_type, ev.is_duplicate_event,
    COALESCE(ev.source_id, '') AS source_id,
    ev.headers, ev.raw, ev.data, ev.created_at,
    COALESCE(ev.idempotency_key, '') AS idempotency_key,
    COALESCE(ev.url_query_params, '') AS url_query_params,
    ev.updated_at, ev.deleted_at, ev.acknowledged_at, ev.metadata, ev.status,
    COALESCE(s.id, '') AS "source_metadata.id",
    COALESCE(s.name, '') AS "source_metadata.name"
FROM convoy.events ev
LEFT JOIN convoy.sources s ON s.id = ev.source_id
WHERE ev.deleted_at IS NULL
    -- EXISTS subquery for endpoint/owner filters (enables index usage)
    AND (
        CASE
            WHEN @has_endpoint_or_owner_filter::boolean THEN -- has_endpoint_or_owner_filter
                EXISTS (
                    SELECT 1
                    FROM convoy.events_endpoints ee
                    JOIN convoy.endpoints e ON e.id = ee.endpoint_id
                    WHERE ee.event_id = ev.id
                        AND (CASE WHEN @has_owner_id::boolean THEN e.owner_id = @owner_id ELSE true END) -- has_owner_id
                        AND (CASE WHEN @has_endpoint_ids::boolean THEN ee.endpoint_id = ANY(@endpoint_ids::text[]) ELSE true END) -- has_endpoint_ids
                )
            ELSE true
        END
    )
    -- Base filters
    AND ev.project_id = @project_id
    AND (CASE WHEN @has_idempotency_key::boolean THEN ev.idempotency_key = @idempotency_key ELSE true END) -- has_idempotency_key
    AND ev.created_at >= @start_date
    AND ev.created_at <= @end_date
    -- Source filter
    AND (CASE WHEN @has_source_ids::boolean THEN ev.source_id = ANY(@source_ids::text[]) ELSE true END) -- has_source_ids
    -- Broker message ID filter
    AND (CASE WHEN @has_broker_message_id::boolean THEN ev.headers -> 'x-broker-message-id' ->> 0 = @broker_message_id ELSE true END) -- has_broker_message_id
    -- Cursor pagination
    AND (CASE WHEN @has_cursor::boolean THEN ev.id <= @cursor ELSE true END) -- has_cursor (for DESC forward or ASC backward)
    AND (CASE WHEN @cursor_gte::boolean THEN ev.id >= @cursor ELSE true END) -- cursor_gte (for ASC forward or DESC backward)
ORDER BY
    CASE WHEN @sort_asc::boolean THEN ev.created_at END ASC,  -- sort_asc
    CASE WHEN @sort_asc::boolean THEN ev.id END ASC,
    CASE WHEN NOT @sort_asc::boolean THEN ev.created_at END DESC,
    CASE WHEN NOT @sort_asc::boolean THEN ev.id END DESC
LIMIT @page_limit; -- limit

-- name: LoadEventsPagedSearch :many
-- Full-text search pagination using CTE + JOIN + GROUP BY
-- Uses convoy.events_search table for search_token matching
WITH events AS (
    SELECT
        ev.id, ev.project_id, ev.event_type, ev.is_duplicate_event,
        COALESCE(ev.source_id, '') AS source_id,
        ev.headers, ev.raw, ev.data, ev.created_at,
        COALESCE(ev.idempotency_key, '') AS idempotency_key,
        COALESCE(ev.url_query_params, '') AS url_query_params,
        ev.updated_at, ev.deleted_at, ev.acknowledged_at, ev.metadata AS metadata, ev.status AS status,
        COALESCE(s.id, '') AS "source_metadata.id",
        COALESCE(s.name, '') AS "source_metadata.name"
    FROM convoy.events_search ev
    LEFT JOIN convoy.events_endpoints ee ON ee.event_id = ev.id
    LEFT JOIN convoy.sources s ON s.id = ev.source_id
    LEFT JOIN convoy.endpoints e ON e.id = ee.endpoint_id
    WHERE ev.deleted_at IS NULL
        -- Base filters
        AND ev.project_id = @project_id
        AND (CASE WHEN @has_idempotency_key::boolean THEN ev.idempotency_key = @idempotency_key ELSE true END) -- has_idempotency_key
        AND ev.created_at >= @start_date
        AND ev.created_at <= @end_date
        -- Source filter
        AND (CASE WHEN @has_source_ids::boolean THEN ev.source_id = ANY(@source_ids::text[]) ELSE true END) -- has_source_ids
        -- Endpoint filter
        AND (CASE WHEN @has_endpoint_ids::boolean THEN ee.endpoint_id = ANY(@endpoint_ids::text[]) ELSE true END) -- has_endpoint_ids
        -- Broker message ID filter
        AND (CASE WHEN @has_broker_message_id::boolean THEN ev.headers -> 'x-broker-message-id' ->> 0 = @broker_message_id ELSE true END) -- has_broker_message_id
        -- Search query filter
        AND (CASE WHEN @has_query::boolean THEN ev.search_token @@ websearch_to_tsquery('simple', @query) ELSE true END) -- has_query
        -- Cursor pagination
        AND (CASE WHEN @has_cursor::boolean THEN ev.id <= @cursor ELSE true END) -- has_cursor (for DESC forward or ASC backward)
        AND (CASE WHEN @cursor_gte::boolean THEN ev.id >= @cursor ELSE true END) -- cursor_gte (for ASC forward or DESC backward)
    GROUP BY ev.id, s.id
    ORDER BY
        CASE WHEN @sort_asc::boolean THEN ev.created_at END ASC,  -- sort_asc
        CASE WHEN @sort_asc::boolean THEN ev.id END ASC,
        CASE WHEN NOT @sort_asc::boolean THEN ev.created_at END DESC,
        CASE WHEN NOT @sort_asc::boolean THEN ev.id END DESC
    LIMIT @page_limit -- limit
)
SELECT id, project_id, event_type, is_duplicate_event, source_id, headers, raw, data, created_at,
       idempotency_key, url_query_params, updated_at, deleted_at, acknowledged_at, metadata, status,
       "source_metadata.id", "source_metadata.name"
FROM events
ORDER BY
    CASE WHEN @sort_asc::boolean THEN created_at END ASC,  -- sort_asc
    CASE WHEN @sort_asc::boolean THEN id END ASC,
    CASE WHEN NOT @sort_asc::boolean THEN created_at END DESC,
    CASE WHEN NOT @sort_asc::boolean THEN id END DESC;

-- name: CountPrevEventsExists :one
-- Check if there are events before cursor (for HasPrevPage) - EXISTS path
SELECT EXISTS(
    SELECT 1
    FROM convoy.events ev
    LEFT JOIN convoy.events_endpoints ee ON ev.id = ee.event_id
    WHERE ev.deleted_at IS NULL
        AND ev.project_id = @project_id
        AND (CASE WHEN @has_idempotency_key::boolean THEN ev.idempotency_key = @idempotency_key ELSE true END) -- has_idempotency_key
        AND ev.created_at >= @start_date
        AND ev.created_at <= @end_date
        -- Source filter
        AND (CASE WHEN @has_source_ids::boolean THEN ev.source_id = ANY(@source_ids::text[]) ELSE true END) -- has_source_ids
        -- Endpoint filter
        AND (CASE WHEN @has_endpoint_ids::boolean THEN ee.endpoint_id = ANY(@endpoint_ids::text[]) ELSE true END) -- has_endpoint_ids
        -- Broker message ID filter
        AND (CASE WHEN @has_broker_message_id::boolean THEN ev.headers -> 'x-broker-message-id' ->> 0 = @broker_message_id ELSE true END) -- has_broker_message_id
        -- Cursor check (> for ASC, < for DESC indicated by sort_asc)
        AND (CASE
            WHEN @sort_asc::boolean THEN ev.id < @cursor  -- sort_asc = true means check for < cursor
            ELSE ev.id > @cursor                     -- sort_asc = false means check for > cursor
        END)
);

-- name: CountPrevEventsSearch :one
-- Check if there are events before cursor (for HasPrevPage) - Search path
SELECT EXISTS(
    SELECT 1
    FROM convoy.events_search ev
    LEFT JOIN convoy.events_endpoints ee ON ev.id = ee.event_id
    WHERE ev.deleted_at IS NULL
        AND ev.project_id = @project_id
        AND (CASE WHEN @has_idempotency_key::boolean THEN ev.idempotency_key = @idempotency_key ELSE true END) -- has_idempotency_key
        AND ev.created_at >= @start_date
        AND ev.created_at <= @end_date
        -- Source filter
        AND (CASE WHEN @has_source_ids::boolean THEN ev.source_id = ANY(@source_ids::text[]) ELSE true END) -- has_source_ids
        -- Endpoint filter
        AND (CASE WHEN @has_endpoint_ids::boolean THEN ee.endpoint_id = ANY(@endpoint_ids::text[]) ELSE true END) -- has_endpoint_ids
        -- Broker message ID filter
        AND (CASE WHEN @has_broker_message_id::boolean THEN ev.headers -> 'x-broker-message-id' ->> 0 = @broker_message_id ELSE true END) -- has_broker_message_id
        -- Search query filter
        AND (CASE WHEN @has_query::boolean THEN ev.search_token @@ websearch_to_tsquery('simple', @query) ELSE true END) -- has_query
        -- Cursor check (> for ASC, < for DESC)
        AND (CASE
            WHEN @sort_asc::boolean THEN ev.id < @cursor  -- sort_asc = true means check for < cursor
            ELSE ev.id > @cursor                     -- sort_asc = false means check for > cursor
        END)
);

-- ============================================================================
-- Group 4: Deletion & Maintenance (4 queries)
-- ============================================================================

-- name: SoftDeleteProjectEvents :exec
UPDATE convoy.events
SET deleted_at = NOW()
WHERE project_id = @project_id
    AND created_at >= @start_date
    AND created_at <= @end_date
    AND deleted_at IS NULL;

-- name: HardDeleteProjectEvents :exec
DELETE FROM convoy.events
WHERE project_id = @project_id
    AND created_at >= @start_date
    AND created_at <= @end_date
    AND NOT EXISTS (
        SELECT 1
        FROM convoy.event_deliveries
        WHERE event_id = convoy.events.id
    );

-- name: HardDeleteTokenizedEvents :exec
DELETE FROM convoy.events_search
WHERE project_id = @project_id;

-- name: CopyRowsFromEventsToEventsSearch :exec
SELECT convoy.copy_rows(@project_id, @batch_size);

-- ============================================================================
-- Group 5: Partition Management (4 queries)
-- ============================================================================
-- TODO: These functions need to be created in the database first
-- For now, implement these methods manually in impl.go using the full SQL strings

-- -- name: PartitionEventsTable :exec
-- SELECT convoy.partition_events_table();

-- -- name: UnPartitionEventsTable :exec
-- SELECT convoy.un_partition_events_table();

-- -- name: PartitionEventsSearchTable :exec
-- SELECT convoy.partition_events_search_table();

-- -- name: UnPartitionEventsSearchTable :exec
-- SELECT convoy.un_partition_events_search_table();
