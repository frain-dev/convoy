package inventory

import (
	"context"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"
)

// Postgres reads only the existing store. Unlike NewQueue it starts no write
// buffers, completion flushers or background processes and owns no DB pool.
type Postgres struct{ DB *sqlx.DB }

const postgresSnapshotSQL = `
WITH counts AS (
 SELECT queue_name AS name,
 COUNT(*) FILTER (WHERE status = 'pending' AND run_at <= NOW()) AS pending,
 COUNT(*) FILTER (WHERE status = 'pending' AND run_at > NOW()) AS future,
 COUNT(*) FILTER (WHERE status = 'processing') AS processing,
 COUNT(*) FILTER (WHERE status = 'archived') AS archived,
 COUNT(*) FILTER (WHERE status = 'completed') AS completed,
 COUNT(*) FILTER (WHERE status IS NULL OR status NOT IN ('pending','processing','archived','completed')) AS unknown,
 COALESCE(EXTRACT(EPOCH FROM NOW() - MIN(run_at) FILTER (WHERE status = 'pending' AND run_at <= NOW())) * 1000, 0)::bigint AS oldest_due_age_ms
 FROM convoy.queue_jobs GROUP BY queue_name
), names AS (
 SELECT name FROM counts UNION SELECT queue_name FROM convoy.queue_state WHERE paused_at IS NOT NULL
)
SELECT n.name, COALESCE(c.pending,0) AS pending, COALESCE(c.future,0) AS future,
 COALESCE(c.processing,0) AS processing, COALESCE(c.archived,0) AS archived,
 COALESCE(c.completed,0) AS completed, COALESCE(c.unknown,0) AS unknown,
 COALESCE(c.oldest_due_age_ms,0) AS oldest_due_age_ms,
 (s.paused_at IS NOT NULL) AS paused
FROM names n LEFT JOIN counts c ON c.name=n.name
LEFT JOIN convoy.queue_state s ON s.queue_name=n.name ORDER BY n.name`

func (p Postgres) Inspect(ctx context.Context) (Snapshot, error) {
	if p.DB == nil {
		return Snapshot{}, errors.New("queue store database is unavailable")
	}
	snapshot := Snapshot{Queues: []Queue{}}
	if err := p.DB.SelectContext(ctx, &snapshot.Queues, postgresSnapshotSQL); err != nil {
		return Snapshot{}, err
	}
	snapshot.ObservedAt = time.Now().UTC()
	return snapshot, nil
}
