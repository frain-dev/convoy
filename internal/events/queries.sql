-- Events Repository SQL Queries
-- This file contains all SQL queries for the events repository
-- Total: 19 queries organized into 5 groups

-- ============================================================================
-- Group 1: Simple CRUD Operations (5 queries)
-- ============================================================================

-- TODO: name: CreateEvent :exec
-- Insert a new event into convoy.events

-- TODO: name: CreateEventEndpoints :exec
-- Insert event-endpoint associations (batch insert with ON CONFLICT DO NOTHING)

-- TODO: name: UpdateEventEndpoints :exec
-- Update event endpoints array

-- TODO: name: UpdateEventStatus :exec
-- Update event status

-- TODO: name: FindEventByID :one
-- Find event by ID and project_id (LEFT JOIN sources for source_metadata)

-- ============================================================================
-- Group 2: Batch Reads & Counting (5 queries)
-- ============================================================================

-- TODO: name: FindEventsByIDs :many
-- Find multiple events by IDs (LEFT JOIN events_endpoints, endpoints, sources)

-- TODO: name: FindEventsByIdempotencyKey :many
-- Find all events with a specific idempotency key

-- TODO: name: FindFirstEventWithIdempotencyKey :one
-- Find the first non-duplicate event with idempotency key

-- TODO: name: CountProjectMessages :one
-- Count total events in a project (WHERE deleted_at IS NULL)

-- TODO: name: CountEvents :one
-- Count events with filters (endpoint_ids, source_id, date range)
-- Uses LEFT JOIN events_endpoints, endpoints

-- ============================================================================
-- Group 3: Complex Pagination (5 queries) ⚠️ MOST CRITICAL
-- ============================================================================

-- TODO: name: LoadEventsPagedExists :many
-- Fast pagination using EXISTS subquery (no search query)
-- Pattern: SELECT ... FROM events WHERE EXISTS (SELECT 1 FROM events_endpoints ...)
-- Filters: project_id, endpoint_ids, source_ids, owner_id, dates, idempotency_key, broker_message_id
-- Pagination: cursor, limit, direction (forward/backward), sort (ASC/DESC)
-- Use CASE expressions for conditional filters

-- TODO: name: LoadEventsPagedSearch :many
-- Full-text search pagination using CTE + JOIN + GROUP BY
-- Uses convoy.events_search table with search_token column
-- Same filters as above + search query

-- TODO: name: CountPrevEventsExists :one
-- Check if there are events before cursor (for HasPrevPage) - EXISTS path
-- Returns boolean: EXISTS(SELECT 1 FROM events WHERE id > cursor ...)

-- TODO: name: CountPrevEventsSearch :one
-- Check if there are events before cursor (for HasPrevPage) - Search path
-- Returns boolean: EXISTS(SELECT 1 FROM events_search WHERE id > cursor ...)

-- ============================================================================
-- Group 4: Deletion & Maintenance (3 queries)
-- ============================================================================

-- TODO: name: SoftDeleteProjectEvents :exec
-- Soft delete events: UPDATE convoy.events SET deleted_at = NOW()
-- WHERE project_id = $1 AND created_at >= $2 AND created_at <= $3

-- TODO: name: HardDeleteProjectEvents :exec
-- Hard delete events: DELETE FROM convoy.events
-- WHERE project_id = $1 AND created_at >= $2 AND created_at <= $3
-- AND NOT EXISTS (SELECT 1 FROM event_deliveries WHERE event_id = events.id)

-- TODO: name: HardDeleteTokenizedEvents :exec
-- Hard delete from events_search: DELETE FROM convoy.events_search WHERE project_id = $1

-- TODO: name: CopyRowsFromEventsToEventsSearch :exec
-- Call PL/pgSQL function: SELECT convoy.copy_rows($1, $2)

-- ============================================================================
-- Group 5: Partition Management (4 queries)
-- ============================================================================

-- TODO: name: PartitionEventsTable :exec
-- Call PL/pgSQL function: select convoy.partition_events_table()

-- TODO: name: UnPartitionEventsTable :exec
-- Call PL/pgSQL function: select convoy.un_partition_events_table()

-- TODO: name: PartitionEventsSearchTable :exec
-- Call PL/pgSQL function: select convoy.partition_events_search_table()

-- TODO: name: UnPartitionEventsSearchTable :exec
-- Call PL/pgSQL function: select convoy.un_partition_events_search_table()

-- ============================================================================
-- Implementation Notes
-- ============================================================================

-- 1. Use CASE expressions for conditional filters:
--    AND (CASE WHEN @has_endpoint_ids::boolean THEN ee.endpoint_id = ANY(@endpoint_ids::text[]) ELSE true END)
--
-- 2. For pagination, use:
--    - Cursor-based: WHERE id <= @cursor (DESC) or id >= @cursor (ASC)
--    - LIMIT @limit + 1 (for hasNext detection)
--
-- 3. For dual query path (EXISTS vs CTE), create separate queries:
--    - LoadEventsPagedExists: Fast, no GROUP BY, uses index
--    - LoadEventsPagedSearch: Full-text search with GROUP BY
--
-- 4. Handle nullable fields with COALESCE:
--    COALESCE(source_id, '') AS source_id
--
-- 5. For source metadata, use LEFT JOIN:
--    COALESCE(s.id, '') AS "source_metadata.id"
--    COALESCE(s.name, '') AS "source_metadata.name"
