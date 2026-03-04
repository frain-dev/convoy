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
    $1, $2, $3, $4, $5,
    $6, $7, $8, $9, $10,
    $11, $12, $13, $14
);

-- name: CreateEventEndpoints :exec
INSERT INTO convoy.events_endpoints (event_id, endpoint_id)
VALUES ($1, $2)
ON CONFLICT (endpoint_id, event_id) DO NOTHING;

-- name: UpdateEventEndpoints :exec
UPDATE convoy.events
SET endpoints = $1
WHERE project_id = $2 AND id = $3;

-- name: UpdateEventStatus :exec
UPDATE convoy.events
SET status = $1
WHERE project_id = $2 AND id = $3;

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
WHERE ev.id = $1 AND ev.project_id = $2 AND ev.deleted_at IS NULL;

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
    AND ev.id = ANY($1::text[])
    AND ev.project_id = $2;

-- name: FindEventsByIdempotencyKey :many
SELECT id
FROM convoy.events
WHERE idempotency_key = $1
    AND project_id = $2
    AND deleted_at IS NULL;

-- name: FindFirstEventWithIdempotencyKey :one
SELECT id
FROM convoy.events
WHERE idempotency_key = $1
    AND is_duplicate_event IS FALSE
    AND project_id = $2
    AND deleted_at IS NULL
ORDER BY created_at
LIMIT 1;

-- name: CountProjectMessages :one
SELECT COUNT(project_id)
FROM convoy.events
WHERE project_id = $1 AND deleted_at IS NULL;

-- name: CountEvents :one
SELECT COUNT(DISTINCT(ev.id))
FROM convoy.events ev
LEFT JOIN convoy.events_endpoints ee ON ee.event_id = ev.id
LEFT JOIN convoy.endpoints e ON ee.endpoint_id = e.id
WHERE ev.project_id = $1
    AND ev.created_at >= $2
    AND ev.created_at <= $3
    AND ev.deleted_at IS NULL
    AND (CASE WHEN $4::boolean THEN e.id = ANY($5::text[]) ELSE true END)
    AND (CASE WHEN $6::boolean THEN ev.source_id = $7 ELSE true END);

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
            WHEN $1::boolean THEN -- has_endpoint_or_owner_filter
                EXISTS (
                    SELECT 1
                    FROM convoy.events_endpoints ee
                    JOIN convoy.endpoints e ON e.id = ee.endpoint_id
                    WHERE ee.event_id = ev.id
                        AND (CASE WHEN $2::boolean THEN e.owner_id = $3 ELSE true END) -- has_owner_id
                        AND (CASE WHEN $4::boolean THEN ee.endpoint_id = ANY($5::text[]) ELSE true END) -- has_endpoint_ids
                )
            ELSE true
        END
    )
    -- Base filters
    AND ev.project_id = $6
    AND (CASE WHEN $7::boolean THEN ev.idempotency_key = $8 ELSE true END) -- has_idempotency_key
    AND ev.created_at >= $9
    AND ev.created_at <= $10
    -- Source filter
    AND (CASE WHEN $11::boolean THEN ev.source_id = ANY($12::text[]) ELSE true END) -- has_source_ids
    -- Broker message ID filter
    AND (CASE WHEN $13::boolean THEN ev.headers -> 'x-broker-message-id' ->> 0 = $14 ELSE true END) -- has_broker_message_id
    -- Cursor pagination
    AND (CASE WHEN $15::boolean THEN ev.id <= $16 ELSE true END) -- has_cursor (for DESC forward or ASC backward)
    AND (CASE WHEN $17::boolean THEN ev.id >= $16 ELSE true END) -- cursor_gte (for ASC forward or DESC backward)
ORDER BY
    CASE WHEN $18::boolean THEN ev.created_at END ASC,  -- sort_asc
    CASE WHEN $18::boolean THEN ev.id END ASC,
    CASE WHEN NOT $18::boolean THEN ev.created_at END DESC,
    CASE WHEN NOT $18::boolean THEN ev.id END DESC
LIMIT $19; -- limit

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
        AND ev.project_id = $1
        AND (CASE WHEN $2::boolean THEN ev.idempotency_key = $3 ELSE true END) -- has_idempotency_key
        AND ev.created_at >= $4
        AND ev.created_at <= $5
        -- Source filter
        AND (CASE WHEN $6::boolean THEN ev.source_id = ANY($7::text[]) ELSE true END) -- has_source_ids
        -- Endpoint filter
        AND (CASE WHEN $8::boolean THEN ee.endpoint_id = ANY($9::text[]) ELSE true END) -- has_endpoint_ids
        -- Broker message ID filter
        AND (CASE WHEN $10::boolean THEN ev.headers -> 'x-broker-message-id' ->> 0 = $11 ELSE true END) -- has_broker_message_id
        -- Search query filter
        AND (CASE WHEN $12::boolean THEN ev.search_token @@ websearch_to_tsquery('simple', $13) ELSE true END) -- has_query
        -- Cursor pagination
        AND (CASE WHEN $14::boolean THEN ev.id <= $15 ELSE true END) -- has_cursor (for DESC forward or ASC backward)
        AND (CASE WHEN $16::boolean THEN ev.id >= $15 ELSE true END) -- cursor_gte (for ASC forward or DESC backward)
    GROUP BY ev.id, s.id
    ORDER BY
        CASE WHEN $17::boolean THEN ev.created_at END ASC,  -- sort_asc
        CASE WHEN $17::boolean THEN ev.id END ASC,
        CASE WHEN NOT $17::boolean THEN ev.created_at END DESC,
        CASE WHEN NOT $17::boolean THEN ev.id END DESC
    LIMIT $18 -- limit
)
SELECT id, project_id, event_type, is_duplicate_event, source_id, headers, raw, data, created_at,
       idempotency_key, url_query_params, updated_at, deleted_at, acknowledged_at, metadata, status,
       "source_metadata.id", "source_metadata.name"
FROM events
ORDER BY
    CASE WHEN $17::boolean THEN created_at END ASC,  -- sort_asc
    CASE WHEN $17::boolean THEN id END ASC,
    CASE WHEN NOT $17::boolean THEN created_at END DESC,
    CASE WHEN NOT $17::boolean THEN id END DESC;

-- name: CountPrevEventsExists :one
-- Check if there are events before cursor (for HasPrevPage) - EXISTS path
SELECT EXISTS(
    SELECT 1
    FROM convoy.events ev
    LEFT JOIN convoy.events_endpoints ee ON ev.id = ee.event_id
    WHERE ev.deleted_at IS NULL
        AND ev.project_id = $1
        AND (CASE WHEN $2::boolean THEN ev.idempotency_key = $3 ELSE true END) -- has_idempotency_key
        AND ev.created_at >= $4
        AND ev.created_at <= $5
        -- Source filter
        AND (CASE WHEN $6::boolean THEN ev.source_id = ANY($7::text[]) ELSE true END) -- has_source_ids
        -- Endpoint filter
        AND (CASE WHEN $8::boolean THEN ee.endpoint_id = ANY($9::text[]) ELSE true END) -- has_endpoint_ids
        -- Broker message ID filter
        AND (CASE WHEN $10::boolean THEN ev.headers -> 'x-broker-message-id' ->> 0 = $11 ELSE true END) -- has_broker_message_id
        -- Cursor check (> for ASC, < for DESC indicated by sort_asc)
        AND (CASE
            WHEN $12::boolean THEN ev.id < $13  -- sort_asc = true means check for < cursor
            ELSE ev.id > $13                     -- sort_asc = false means check for > cursor
        END)
);

-- name: CountPrevEventsSearch :one
-- Check if there are events before cursor (for HasPrevPage) - Search path
SELECT EXISTS(
    SELECT 1
    FROM convoy.events_search ev
    LEFT JOIN convoy.events_endpoints ee ON ev.id = ee.event_id
    WHERE ev.deleted_at IS NULL
        AND ev.project_id = $1
        AND (CASE WHEN $2::boolean THEN ev.idempotency_key = $3 ELSE true END) -- has_idempotency_key
        AND ev.created_at >= $4
        AND ev.created_at <= $5
        -- Source filter
        AND (CASE WHEN $6::boolean THEN ev.source_id = ANY($7::text[]) ELSE true END) -- has_source_ids
        -- Endpoint filter
        AND (CASE WHEN $8::boolean THEN ee.endpoint_id = ANY($9::text[]) ELSE true END) -- has_endpoint_ids
        -- Broker message ID filter
        AND (CASE WHEN $10::boolean THEN ev.headers -> 'x-broker-message-id' ->> 0 = $11 ELSE true END) -- has_broker_message_id
        -- Search query filter
        AND (CASE WHEN $12::boolean THEN ev.search_token @@ websearch_to_tsquery('simple', $13) ELSE true END) -- has_query
        -- Cursor check (> for ASC, < for DESC)
        AND (CASE
            WHEN $14::boolean THEN ev.id < $15  -- sort_asc = true means check for < cursor
            ELSE ev.id > $15                     -- sort_asc = false means check for > cursor
        END)
);

-- ============================================================================
-- Group 4: Deletion & Maintenance (4 queries)
-- ============================================================================

-- name: SoftDeleteProjectEvents :exec
UPDATE convoy.events
SET deleted_at = NOW()
WHERE project_id = $1
    AND created_at >= $2
    AND created_at <= $3
    AND deleted_at IS NULL;

-- name: HardDeleteProjectEvents :exec
DELETE FROM convoy.events
WHERE project_id = $1
    AND created_at >= $2
    AND created_at <= $3
    AND NOT EXISTS (
        SELECT 1
        FROM convoy.event_deliveries
        WHERE event_id = convoy.events.id
    );

-- name: HardDeleteTokenizedEvents :exec
DELETE FROM convoy.events_search
WHERE project_id = $1;

-- name: CopyRowsFromEventsToEventsSearch :exec
SELECT convoy.copy_rows($1, $2);

-- ============================================================================
-- Group 5: Partition Management (4 queries)
-- ============================================================================

-- name: PartitionEventsTable :exec
SELECT convoy.partition_events_table();

-- name: UnPartitionEventsTable :exec
SELECT convoy.un_partition_events_table();

-- name: PartitionEventsSearchTable :exec
SELECT convoy.partition_events_search_table();

-- name: UnPartitionEventsSearchTable :exec
SELECT convoy.un_partition_events_search_table();
