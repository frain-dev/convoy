-- Delivery Attempts Queries

-- name: CreateDeliveryAttempt :exec
INSERT INTO convoy.delivery_attempts (
    id, url, method, api_version, endpoint_id, event_delivery_id, project_id,
    ip_address, request_http_header, response_http_header, http_status, response_data, error, status
)
VALUES (@id, @url, @method, @api_version, @endpoint_id, @event_delivery_id, @project_id,
        @ip_address, @request_http_header, @response_http_header, @http_status, @response_data, @error, @status);

-- name: FindDeliveryAttemptById :one
SELECT
    id,
    url,
    method,
    api_version,
    endpoint_id,
    event_delivery_id,
    project_id,
    ip_address,
    request_http_header,
    response_http_header,
    http_status,
    response_data,
    error,
    status,
    created_at,
    updated_at,
    deleted_at
FROM convoy.delivery_attempts
WHERE id = @id AND event_delivery_id = @event_delivery_id AND deleted_at IS NULL;

-- name: FindDeliveryAttempts :many
-- Fetch last 10 delivery attempts for an event delivery, ordered by created_at
WITH att AS (
    SELECT
        id,
        url,
        method,
        api_version,
        endpoint_id,
        event_delivery_id,
        project_id,
        ip_address,
        request_http_header,
        response_http_header,
        http_status,
        response_data,
        error,
        status,
        created_at,
        updated_at,
        deleted_at
    FROM convoy.delivery_attempts
    WHERE event_delivery_id = @event_delivery_id AND deleted_at IS NULL
    ORDER BY created_at DESC
    LIMIT 10
)
SELECT * FROM att ORDER BY created_at ASC;

-- name: SoftDeleteProjectDeliveryAttempts :execresult
UPDATE convoy.delivery_attempts
SET deleted_at = NOW()
WHERE project_id = @project_id
    AND created_at >= @created_at_start
    AND created_at <= @created_at_end
    AND deleted_at IS NULL;

-- name: HardDeleteProjectDeliveryAttempts :execresult
DELETE FROM convoy.delivery_attempts
WHERE project_id = @project_id
    AND created_at >= @created_at_start
    AND created_at <= @created_at_end;

-- name: GetFailureAndSuccessCounts :many
-- Get failure and success counts for endpoints within the lookback duration
-- This replaces the n+1 query pattern in the legacy implementation
SELECT
    endpoint_id AS key,
    project_id AS tenant_id,
    COUNT(CASE WHEN status = false THEN 1 END)::bigint AS failures,
    COUNT(CASE WHEN status = true THEN 1 END)::bigint AS successes
FROM convoy.delivery_attempts
WHERE deleted_at IS NULL
    AND created_at >= NOW() - MAKE_INTERVAL(mins := @look_back_duration)
GROUP BY endpoint_id, project_id;

-- name: GetFailureAndSuccessCountsWithResetTime :one
-- Get counts for a specific endpoint from a specific reset time
-- This is used when circuit breaker has been reset for specific endpoints
SELECT
    endpoint_id AS key,
    project_id AS tenant_id,
    COUNT(CASE WHEN status = false THEN 1 END)::bigint AS failures,
    COUNT(CASE WHEN status = true THEN 1 END)::bigint AS successes
FROM convoy.delivery_attempts
WHERE endpoint_id = @endpoint_id
    AND deleted_at IS NULL
    AND created_at >= @reset_time
GROUP BY endpoint_id, project_id;
