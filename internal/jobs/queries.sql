-- Jobs Repository SQL Queries
-- Schema: convoy.jobs
-- Columns: id, type, status, project_id, started_at, completed_at, failed_at, created_at, updated_at, deleted_at

-- name: CreateJob :exec
INSERT INTO convoy.jobs (id, type, status, project_id)
VALUES (@id, @type, @status, @project_id);

-- name: MarkJobAsStarted :execresult
UPDATE convoy.jobs SET
    status = 'running',
    started_at = NOW(),
    updated_at = NOW()
WHERE id = @id AND project_id = @project_id AND deleted_at IS NULL;

-- name: MarkJobAsCompleted :execresult
UPDATE convoy.jobs SET
    status = 'completed',
    completed_at = NOW(),
    updated_at = NOW()
WHERE id = @id AND project_id = @project_id AND deleted_at IS NULL;

-- name: MarkJobAsFailed :execresult
UPDATE convoy.jobs SET
    status = 'failed',
    failed_at = NOW(),
    updated_at = NOW()
WHERE id = @id AND project_id = @project_id AND deleted_at IS NULL;

-- name: DeleteJob :execresult
UPDATE convoy.jobs SET
    deleted_at = NOW()
WHERE id = @id AND project_id = @project_id AND deleted_at IS NULL;

-- name: FetchJobById :one
SELECT id, type, status, project_id, started_at, completed_at, failed_at, created_at, updated_at, deleted_at
FROM convoy.jobs
WHERE id = @id AND project_id = @project_id AND deleted_at IS NULL;

-- name: FetchRunningJobsByProjectId :many
SELECT id, type, status, project_id, started_at, completed_at, failed_at, created_at, updated_at, deleted_at
FROM convoy.jobs
WHERE status = 'running' AND project_id = @project_id AND deleted_at IS NULL;

-- name: FetchJobsByProjectId :many
SELECT id, type, status, project_id, started_at, completed_at, failed_at, created_at, updated_at, deleted_at
FROM convoy.jobs
WHERE project_id = @project_id AND deleted_at IS NULL;

-- name: FetchJobsPaginated :many
WITH filtered_jobs AS (
    SELECT id, type, status, project_id,
           started_at, completed_at, failed_at,
           created_at, updated_at
    FROM convoy.jobs
    WHERE deleted_at IS NULL
      AND project_id = @project_id
      AND (
        CASE
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
SELECT * FROM filtered_jobs
ORDER BY
  CASE WHEN @direction::text = 'prev' THEN id END DESC,
  CASE WHEN @direction::text = 'next' THEN id END DESC;

-- name: CountPrevJobs :one
SELECT COALESCE(COUNT(DISTINCT(id)), 0) AS count
FROM convoy.jobs
WHERE deleted_at IS NULL
  AND project_id = @project_id
  AND id > @cursor;
