-- Batch Retry Repository SQLc Queries
-- This file contains all SQL queries for batch retry operations

-- name: CreateBatchRetry :exec
INSERT INTO convoy.batch_retries (
    id, project_id, status, total_events, processed_events, failed_events,
    filter, created_at, updated_at, completed_at, error
) VALUES (
    @id, @project_id, @status, @total_events, @processed_events, @failed_events,
    @filter, @created_at, @updated_at, @completed_at, @error
);

-- name: UpdateBatchRetry :execresult
UPDATE convoy.batch_retries SET
    status = @status,
    processed_events = @processed_events,
    failed_events = @failed_events,
    updated_at = @updated_at,
    filter = @filter,
    total_events = @total_events,
    completed_at = @completed_at,
    error = @error
WHERE id = @id AND project_id = @project_id;

-- name: FindBatchRetryByID :one
SELECT * FROM convoy.batch_retries WHERE id = @id;

-- name: FindActiveBatchRetry :one
SELECT * FROM convoy.batch_retries
WHERE project_id = @project_id
AND status IN (@status1, @status2)
ORDER BY created_at DESC
LIMIT 1;
