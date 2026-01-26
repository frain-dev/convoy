-- Filters Queries
-- This file contains SQLc queries for the filters service

-- name: CreateFilter :exec
INSERT INTO convoy.filters (
    id, subscription_id, event_type,
    headers, body, raw_headers, raw_body,
    created_at, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: FindFilterByID :one
SELECT
    id, subscription_id, event_type,
    headers, body, raw_headers, raw_body,
    created_at, updated_at
FROM convoy.filters
WHERE id = $1;

-- name: FindFiltersBySubscriptionID :many
SELECT
    id, subscription_id, event_type,
    headers, body, raw_headers, raw_body,
    created_at, updated_at
FROM convoy.filters
WHERE subscription_id = $1
ORDER BY created_at DESC;

-- name: FindFilterBySubscriptionAndEventType :one
SELECT
    id, subscription_id, event_type,
    headers, body, raw_headers, raw_body,
    created_at, updated_at
FROM convoy.filters
WHERE subscription_id = $1 AND event_type = $2;

-- name: UpdateFilter :execrows
UPDATE convoy.filters
SET
    headers = $2,
    body = $3,
    raw_headers = $4,
    raw_body = $5,
    event_type = $6,
    updated_at = $7
WHERE id = $1;

-- name: DeleteFilter :execrows
DELETE FROM convoy.filters
WHERE id = $1;
