-- name: EnqueueBackupJob :exec
INSERT INTO convoy.backup_jobs (project_id, hour_start, hour_end, status)
VALUES (@project_id, @hour_start, @hour_end, 'pending')
ON CONFLICT (project_id, hour_start) DO NOTHING;

-- name: ClaimBackupJob :one
UPDATE convoy.backup_jobs
SET status = 'claimed', worker_id = sqlc.arg(worker_id), claimed_at = NOW()
WHERE id = (
    SELECT id FROM convoy.backup_jobs
    WHERE status = 'pending'
    ORDER BY created_at ASC
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
RETURNING id, project_id, hour_start, hour_end, status, worker_id, claimed_at, completed_at, error, record_counts, created_at, updated_at;

-- name: CompleteBackupJob :exec
UPDATE convoy.backup_jobs
SET status = 'completed', completed_at = NOW(), record_counts = @record_counts
WHERE id = @id;

-- name: FailBackupJob :exec
UPDATE convoy.backup_jobs
SET status = 'failed', error = @error, completed_at = NOW()
WHERE id = @id;

-- name: ReclaimStaleJobs :execresult
UPDATE convoy.backup_jobs
SET status = 'pending', worker_id = NULL, claimed_at = NULL
WHERE status = 'claimed' AND claimed_at < NOW() - MAKE_INTERVAL(mins := @stale_minutes);

-- name: FindLatestCompletedBackup :one
SELECT id, project_id, hour_start, hour_end, status, worker_id, claimed_at, completed_at, error, record_counts, created_at, updated_at
FROM convoy.backup_jobs
WHERE project_id = sqlc.arg(project_id) AND status = 'completed'
ORDER BY hour_start DESC
LIMIT 1;
