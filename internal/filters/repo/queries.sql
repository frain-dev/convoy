-- Filters Queries
-- This file contains SQLc queries for the filters service

-- name: CreateFilter :exec
INSERT INTO convoy.filters (
    id, subscription_id, event_type,
    headers, body, raw_headers, raw_body,
    created_at, updated_at
)
VALUES (@id, @subscription_id, @event_type, @headers, @body, @raw_headers, @raw_body, @created_at, @updated_at);

-- name: FindFilterByID :one
SELECT
    id, subscription_id, event_type,
    headers, body, raw_headers, raw_body,
    created_at, updated_at
FROM convoy.filters
WHERE id = @id;

-- name: FindFiltersBySubscriptionID :many
SELECT
    id, subscription_id, event_type,
    headers, body, raw_headers, raw_body,
    created_at, updated_at
FROM convoy.filters
WHERE subscription_id = @subscription_id
ORDER BY created_at DESC;

-- name: FindFilterBySubscriptionAndEventType :one
SELECT
    id, subscription_id, event_type,
    headers, body, raw_headers, raw_body,
    created_at, updated_at
FROM convoy.filters
WHERE subscription_id = @subscription_id AND event_type = @event_type;

-- name: UpdateFilter :execrows
UPDATE convoy.filters
SET
    headers = @headers,
    body = @body,
    raw_headers = @raw_headers,
    raw_body = @raw_body,
    event_type = @event_type,
    updated_at = @updated_at
WHERE id = @id;

-- name: DeleteFilter :execrows
DELETE FROM convoy.filters
WHERE id = @id;
