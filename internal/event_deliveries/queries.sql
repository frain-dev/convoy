-- Event Deliveries Repository SQL Queries
-- Migrated from database/postgres/event_delivery.go to sqlc

-- ============================================================================
-- Group 1: CRUD Operations
-- ============================================================================

-- name: CreateEventDelivery :exec
INSERT INTO convoy.event_deliveries (
    id, project_id, event_id, endpoint_id, device_id, subscription_id, headers, status,
    metadata, cli_metadata, description, url_query_params, idempotency_key, event_type, acknowledged_at, delivery_mode
)
VALUES (@id, @project_id, @event_id, @endpoint_id, @device_id, @subscription_id, @headers, @status,
        @metadata, @cli_metadata, @description, @url_query_params, @idempotency_key, @event_type, @acknowledged_at, @delivery_mode);

-- name: UpdateEventDeliveryMetadata :exec
UPDATE convoy.event_deliveries
SET status = @status, metadata = @metadata, latency_seconds = @latency_seconds, updated_at = NOW()
WHERE id = @id AND project_id = @project_id AND deleted_at IS NULL;

-- name: UpdateStatusOfEventDeliveries :exec
UPDATE convoy.event_deliveries
SET status = @status, description = @description, updated_at = NOW()
WHERE (project_id = @project_id OR @project_id = '')
  AND id = ANY(@ids::TEXT[])
  AND deleted_at IS NULL;

-- ============================================================================
-- Group 2: Find Operations
-- ============================================================================

-- name: FindEventDeliveryByID :one
SELECT
    ed.id, ed.project_id, ed.event_id, ed.subscription_id,
    ed.headers, ed.attempts, ed.status, ed.metadata, ed.cli_metadata,
    COALESCE(ed.url_query_params, '') AS url_query_params,
    COALESCE(ed.idempotency_key, '') AS idempotency_key,
    ed.description, ed.created_at, ed.updated_at, ed.acknowledged_at,
    COALESCE(ed.event_type, '') AS event_type,
    COALESCE(ed.device_id, '') AS device_id,
    COALESCE(ed.endpoint_id, '') AS endpoint_id,
    COALESCE(ed.delivery_mode, 'at_least_once')::TEXT AS delivery_mode,
    COALESCE(ed.latency_seconds, 0) AS latency_seconds,
    COALESCE(ep.id, '') AS "endpoint_metadata.id",
    COALESCE(ep.name, '') AS "endpoint_metadata.name",
    COALESCE(ep.project_id, '') AS "endpoint_metadata.project_id",
    COALESCE(ep.support_email, '') AS "endpoint_metadata.support_email",
    COALESCE(ep.url, '') AS "endpoint_metadata.url",
    COALESCE(ep.owner_id, '') AS "endpoint_metadata.owner_id",
    ev.id AS "event_metadata.id",
    ev.event_type AS "event_metadata.event_type",
    COALESCE(d.id, '') AS "device_metadata.id",
    COALESCE(d.status, '') AS "device_metadata.status",
    COALESCE(d.host_name, '') AS "device_metadata.host_name",
    COALESCE(s.id, '') AS "source_metadata.id",
    COALESCE(s.name, '') AS "source_metadata.name"
FROM convoy.event_deliveries ed
LEFT JOIN convoy.endpoints ep ON ed.endpoint_id = ep.id
LEFT JOIN convoy.events ev ON ed.event_id = ev.id
LEFT JOIN convoy.devices d ON ed.device_id = d.id
LEFT JOIN convoy.sources s ON s.id = ev.source_id
WHERE ed.deleted_at IS NULL
  AND ed.id = @id AND ed.project_id = @project_id;

-- name: FindEventDeliveryByIDSlim :one
SELECT
    id, project_id, event_id, subscription_id,
    headers, attempts, status, metadata, cli_metadata,
    COALESCE(url_query_params, '') AS url_query_params,
    COALESCE(idempotency_key, '') AS idempotency_key,
    created_at, updated_at,
    COALESCE(event_type, '') AS event_type,
    COALESCE(device_id, '') AS device_id,
    COALESCE(endpoint_id, '') AS endpoint_id,
    COALESCE(delivery_mode, 'at_least_once')::TEXT AS delivery_mode,
    acknowledged_at
FROM convoy.event_deliveries
WHERE deleted_at IS NULL
  AND project_id = @project_id AND id = @id;

-- name: FindEventDeliveriesByIDs :many
SELECT
    id, project_id, event_id, subscription_id,
    headers, attempts, status, metadata, cli_metadata,
    COALESCE(idempotency_key, '') AS idempotency_key,
    COALESCE(url_query_params, '') AS url_query_params,
    description, created_at, updated_at,
    COALESCE(event_type, '') AS event_type,
    COALESCE(device_id, '') AS device_id,
    COALESCE(endpoint_id, '') AS endpoint_id,
    COALESCE(delivery_mode, 'at_least_once')::TEXT AS delivery_mode,
    acknowledged_at
FROM convoy.event_deliveries
WHERE id = ANY(@ids::TEXT[])
  AND project_id = @project_id
  AND deleted_at IS NULL;

-- name: FindEventDeliveriesByEventID :many
SELECT
    id, project_id, event_id, subscription_id,
    headers, attempts, status, metadata, cli_metadata,
    COALESCE(idempotency_key, '') AS idempotency_key,
    COALESCE(url_query_params, '') AS url_query_params,
    description, created_at, updated_at,
    COALESCE(event_type, '') AS event_type,
    COALESCE(device_id, '') AS device_id,
    COALESCE(endpoint_id, '') AS endpoint_id,
    COALESCE(delivery_mode, 'at_least_once')::TEXT AS delivery_mode,
    acknowledged_at
FROM convoy.event_deliveries
WHERE event_id = @event_id
  AND project_id = @project_id
  AND deleted_at IS NULL;

-- name: FindDiscardedEventDeliveries :many
SELECT
    id, project_id, event_id, subscription_id,
    headers, attempts, status, metadata, cli_metadata,
    COALESCE(idempotency_key, '') AS idempotency_key,
    COALESCE(url_query_params, '') AS url_query_params,
    description, created_at, updated_at,
    COALESCE(event_type, '') AS event_type,
    COALESCE(device_id, '') AS device_id,
    COALESCE(delivery_mode, 'at_least_once')::TEXT AS delivery_mode,
    acknowledged_at
FROM convoy.event_deliveries
WHERE status = 'Discarded'
  AND project_id = @project_id
  AND device_id = @device_id
  AND created_at >= @start_date
  AND created_at <= @end_date
  AND deleted_at IS NULL;

-- name: FindStuckEventDeliveriesByStatus :many
SELECT id, project_id
FROM convoy.event_deliveries
WHERE status = @status
  AND created_at <= now() - make_interval(secs := 30)
  AND deleted_at IS NULL
FOR UPDATE SKIP LOCKED
LIMIT 1000;

-- ============================================================================
-- Group 3: Count Operations
-- ============================================================================

-- name: CountDeliveriesByStatus :one
SELECT COUNT(id) AS count
FROM convoy.event_deliveries
WHERE status = @status
  AND (project_id = @project_id OR @project_id = '')
  AND created_at >= @start_date
  AND created_at <= @end_date
  AND deleted_at IS NULL;

-- name: CountEventDeliveries :one
SELECT COUNT(id) AS count
FROM convoy.event_deliveries
WHERE (project_id = @project_id OR @project_id = '')
  AND (event_id = @event_id OR @event_id = '')
  AND created_at >= @start_date
  AND created_at <= @end_date
  AND deleted_at IS NULL
  AND (CASE WHEN @has_endpoint_ids::BOOLEAN THEN endpoint_id = ANY(@endpoint_ids::TEXT[]) ELSE true END)
  AND (CASE WHEN @has_status::BOOLEAN THEN status = ANY(@statuses::TEXT[]) ELSE true END);

-- ============================================================================
-- Group 4: Pagination
-- ============================================================================

-- name: LoadEventDeliveriesPaged :many
WITH filtered_deliveries AS (
    SELECT
        ed.id, ed.project_id, ed.event_id, ed.subscription_id,
        ed.headers, ed.attempts, ed.status, ed.metadata, ed.cli_metadata,
        COALESCE(ed.url_query_params, '') AS url_query_params,
        COALESCE(ed.idempotency_key, '') AS idempotency_key,
        ed.description, ed.created_at, ed.updated_at, ed.acknowledged_at,
        COALESCE(ed.event_type, '') AS event_type,
        COALESCE(ed.device_id, '') AS device_id,
        COALESCE(ed.endpoint_id, '') AS endpoint_id,
        COALESCE(ed.delivery_mode, 'at_least_once')::TEXT AS delivery_mode,
        COALESCE(ed.latency_seconds, 0) AS latency_seconds,
        COALESCE(ep.id, '') AS "endpoint_metadata.id",
        COALESCE(ep.name, '') AS "endpoint_metadata.name",
        COALESCE(ep.project_id, '') AS "endpoint_metadata.project_id",
        COALESCE(ep.support_email, '') AS "endpoint_metadata.support_email",
        COALESCE(ep.url, '') AS "endpoint_metadata.url",
        COALESCE(ep.owner_id, '') AS "endpoint_metadata.owner_id",
        ev.id AS "event_metadata.id",
        ev.event_type AS "event_metadata.event_type",
        COALESCE(d.id, '') AS "device_metadata.id",
        COALESCE(d.status, '') AS "device_metadata.status",
        COALESCE(d.host_name, '') AS "device_metadata.host_name",
        COALESCE(s.id, '') AS "source_metadata.id",
        COALESCE(s.name, '') AS "source_metadata.name",
        COALESCE(s.idempotency_keys, '{}') AS "source_metadata.idempotency_keys"
    FROM convoy.event_deliveries ed
    LEFT JOIN convoy.endpoints ep ON ed.endpoint_id = ep.id
    LEFT JOIN convoy.events ev ON ed.event_id = ev.id
    LEFT JOIN convoy.devices d ON ed.device_id = d.id
    LEFT JOIN convoy.sources s ON s.id = ev.source_id
    WHERE ed.deleted_at IS NULL
      AND (ed.project_id = @project_id OR @project_id = '')
      AND (ed.event_id = @event_id OR @event_id = '')
      AND (ed.event_type = @event_type OR @event_type = '')
      AND ed.created_at >= @start_date
      AND ed.created_at <= @end_date
      -- Endpoint filter
      AND (CASE WHEN @has_endpoint_ids::BOOLEAN THEN ed.endpoint_id = ANY(@endpoint_ids::TEXT[]) ELSE true END)
      -- Status filter
      AND (CASE WHEN @has_status::BOOLEAN THEN ed.status = ANY(@statuses::TEXT[]) ELSE true END)
      -- Subscription filter
      AND (CASE WHEN @has_subscription_id::BOOLEAN THEN ed.subscription_id = @subscription_id ELSE true END)
      -- Broker message ID filter
      AND (CASE WHEN @has_broker_message_id::BOOLEAN THEN ed.headers -> 'x-broker-message-id' ->> 0 = @broker_message_id ELSE true END)
      -- Idempotency key filter
      AND (CASE WHEN @has_idempotency_key::BOOLEAN THEN ed.idempotency_key = @idempotency_key ELSE true END)
      -- Cursor pagination
      AND (
        CASE
            WHEN @cursor = '' THEN true
            WHEN (@sort_order::text = 'DESC' AND @direction::text = 'next') OR (@sort_order::text = 'ASC' AND @direction::text = 'prev') THEN ed.id <= @cursor
            WHEN (@sort_order::text = 'ASC' AND @direction::text = 'next') OR (@sort_order::text = 'DESC' AND @direction::text = 'prev') THEN ed.id >= @cursor
            ELSE true
        END
      )
    ORDER BY
        CASE WHEN (@sort_order::text = 'DESC' AND @direction::text = 'next') OR (@sort_order::text = 'ASC' AND @direction::text = 'prev') THEN ed.id END DESC,
        CASE WHEN (@sort_order::text = 'ASC' AND @direction::text = 'next') OR (@sort_order::text = 'DESC' AND @direction::text = 'prev') THEN ed.id END ASC
    LIMIT @page_limit
)
SELECT id, project_id, event_id, subscription_id,
       headers, attempts, status, metadata, cli_metadata,
       url_query_params, idempotency_key, description,
       created_at, updated_at, acknowledged_at,
       event_type, device_id, endpoint_id, delivery_mode, latency_seconds,
       "endpoint_metadata.id", "endpoint_metadata.name", "endpoint_metadata.project_id",
       "endpoint_metadata.support_email", "endpoint_metadata.url", "endpoint_metadata.owner_id",
       "event_metadata.id", "event_metadata.event_type",
       "device_metadata.id", "device_metadata.status", "device_metadata.host_name",
       "source_metadata.id", "source_metadata.name", "source_metadata.idempotency_keys"
FROM filtered_deliveries
ORDER BY
    CASE WHEN @sort_order::text = 'DESC' THEN id END DESC,
    CASE WHEN @sort_order::text = 'ASC' THEN id END ASC;

-- name: CountPrevEventDeliveries :one
SELECT EXISTS(
    SELECT 1
    FROM convoy.event_deliveries ed
    LEFT JOIN convoy.events ev ON ed.event_id = ev.id
    WHERE ed.deleted_at IS NULL
      AND (ed.project_id = @project_id OR @project_id = '')
      AND (ed.event_id = @event_id OR @event_id = '')
      AND (ed.event_type = @event_type OR @event_type = '')
      AND ed.created_at >= @start_date
      AND ed.created_at <= @end_date
      AND (CASE WHEN @has_endpoint_ids::BOOLEAN THEN ed.endpoint_id = ANY(@endpoint_ids::TEXT[]) ELSE true END)
      AND (CASE WHEN @has_status::BOOLEAN THEN ed.status = ANY(@statuses::TEXT[]) ELSE true END)
      AND (CASE WHEN @has_subscription_id::BOOLEAN THEN ed.subscription_id = @subscription_id ELSE true END)
      AND (CASE WHEN @has_broker_message_id::BOOLEAN THEN ed.headers -> 'x-broker-message-id' ->> 0 = @broker_message_id ELSE true END)
      AND (CASE WHEN @has_idempotency_key::BOOLEAN THEN ed.idempotency_key = @idempotency_key ELSE true END)
      AND (CASE
               WHEN @sort_order::text = 'DESC' THEN ed.id > @cursor
               WHEN @sort_order::text = 'ASC' THEN ed.id < @cursor
               ELSE ed.id > @cursor END)
);

-- ============================================================================
-- Group 5: Intervals
-- ============================================================================

-- name: LoadEventDeliveryIntervalsDaily :many
SELECT
    DATE_TRUNC('day', created_at) AS "data.group_only",
    TO_CHAR(DATE_TRUNC('day', created_at), 'yyyy-mm-dd') AS "data.total_time",
    EXTRACT('doy' FROM created_at) AS "data.index",
    COUNT(*) AS count
FROM convoy.event_deliveries
WHERE project_id = @project_id
  AND deleted_at IS NULL
  AND created_at >= @start_date
  AND created_at <= @end_date
  AND (CASE WHEN @has_endpoint_ids::BOOLEAN THEN endpoint_id = ANY(@endpoint_ids::TEXT[]) ELSE true END)
GROUP BY "data.group_only", "data.index"
ORDER BY "data.group_only" ASC;

-- name: LoadEventDeliveryIntervalsWeekly :many
SELECT
    DATE_TRUNC('week', created_at) AS "data.group_only",
    TO_CHAR(DATE_TRUNC('week', created_at), 'yyyy-mm-dd') AS "data.total_time",
    EXTRACT('week' FROM created_at) AS "data.index",
    COUNT(*) AS count
FROM convoy.event_deliveries
WHERE project_id = @project_id
  AND deleted_at IS NULL
  AND created_at >= @start_date
  AND created_at <= @end_date
  AND (CASE WHEN @has_endpoint_ids::BOOLEAN THEN endpoint_id = ANY(@endpoint_ids::TEXT[]) ELSE true END)
GROUP BY "data.group_only", "data.index"
ORDER BY "data.group_only" ASC;

-- name: LoadEventDeliveryIntervalsMonthly :many
SELECT
    DATE_TRUNC('month', created_at) AS "data.group_only",
    TO_CHAR(DATE_TRUNC('month', created_at), 'yyyy-mm') AS "data.total_time",
    EXTRACT('month' FROM created_at) AS "data.index",
    COUNT(*) AS count
FROM convoy.event_deliveries
WHERE project_id = @project_id
  AND deleted_at IS NULL
  AND created_at >= @start_date
  AND created_at <= @end_date
  AND (CASE WHEN @has_endpoint_ids::BOOLEAN THEN endpoint_id = ANY(@endpoint_ids::TEXT[]) ELSE true END)
GROUP BY "data.group_only", "data.index"
ORDER BY "data.group_only" ASC;

-- name: LoadEventDeliveryIntervalsYearly :many
SELECT
    DATE_TRUNC('year', created_at) AS "data.group_only",
    TO_CHAR(DATE_TRUNC('year', created_at), 'yyyy') AS "data.total_time",
    EXTRACT('year' FROM created_at) AS "data.index",
    COUNT(*) AS count
FROM convoy.event_deliveries
WHERE project_id = @project_id
  AND deleted_at IS NULL
  AND created_at >= @start_date
  AND created_at <= @end_date
  AND (CASE WHEN @has_endpoint_ids::BOOLEAN THEN endpoint_id = ANY(@endpoint_ids::TEXT[]) ELSE true END)
GROUP BY "data.group_only", "data.index"
ORDER BY "data.group_only" ASC;

-- ============================================================================
-- Group 6: Delete Operations
-- ============================================================================

-- name: SoftDeleteProjectEventDeliveries :exec
UPDATE convoy.event_deliveries
SET deleted_at = NOW()
WHERE project_id = @project_id
  AND created_at >= @start_date
  AND created_at <= @end_date
  AND deleted_at IS NULL;

-- name: HardDeleteProjectEventDeliveries :exec
DELETE FROM convoy.event_deliveries
WHERE project_id = @project_id
  AND created_at >= @start_date
  AND created_at <= @end_date;

-- ============================================================================
-- Group 7: Export Operations
-- ============================================================================

-- name: ExportEventDeliveries :many
SELECT ed.id,
       TO_JSONB(ed) - 'id' || JSONB_BUILD_OBJECT('uid', ed.id) AS json_output
FROM convoy.event_deliveries AS ed
WHERE project_id = @project_id
  AND created_at < @created_at
  AND (id > @cursor OR @cursor = '')
  AND deleted_at IS NULL
ORDER BY id
LIMIT @page_limit;

-- name: CountExportedEventDeliveries :one
SELECT COUNT(*) AS count FROM convoy.event_deliveries
WHERE project_id = @project_id
  AND created_at < @created_at
  AND (id > @cursor OR @cursor = '')
  AND deleted_at IS NULL;
