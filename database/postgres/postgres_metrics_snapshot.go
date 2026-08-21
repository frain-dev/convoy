package postgres

import (
	"context"
	"fmt"
	"time"
)

const (
	metricsSnapshotEventQueue           = "event_queue"
	metricsSnapshotEventQueueBacklog    = "event_queue_backlog"
	metricsSnapshotEventDeliveryQueue   = "event_delivery_queue"
	metricsSnapshotEventEndpointBacklog = "event_endpoint_backlog"

	// writeQueueMetricsTimeout bounds the worker that rebuilds Prometheus
	// snapshots. Collect must not use this; it only reads the current
	// generation. The scrape QueryTimeout (default 30s) is why the live
	// GROUP BY never filled gauges.
	writeQueueMetricsTimeout = 10 * time.Minute
)

const eventQueueMetricsSQL = `
SELECT DISTINCT
	project_id,
	COALESCE(source_id, 'http') AS source_id,
	COUNT(*) AS total
FROM convoy.events
GROUP BY project_id, source_id`

const eventQueueBacklogMetricsSQL = `
WITH a1 AS (
	SELECT ed.project_id,
		   COALESCE(e.source_id, 'http') AS source_id,
		   EXTRACT(EPOCH FROM (NOW() - MIN(ed.created_at))) AS age_seconds
	FROM convoy.event_deliveries ed
	LEFT JOIN convoy.events e ON e.id = ed.event_id
	WHERE ed.status = 'Processing'
	GROUP BY ed.project_id, e.source_id
	ORDER BY age_seconds DESC, ed.project_id, e.source_id
	LIMIT 1000
)
SELECT project_id, source_id, age_seconds
FROM (
	SELECT * FROM a1
	UNION ALL
	SELECT ed.project_id,
		   COALESCE(e.source_id, 'http'),
		   0 AS age_seconds
	FROM convoy.event_deliveries ed
	LEFT JOIN convoy.events e ON e.id = ed.event_id
	WHERE ed.status = 'Success'
	  AND NOT EXISTS (
		  SELECT 1 FROM a1
		  WHERE a1.project_id = ed.project_id
			AND a1.source_id = COALESCE(e.source_id, 'http')
	  )
	GROUP BY ed.project_id, e.source_id
) AS combined
ORDER BY project_id, source_id
LIMIT 1000`

const eventDeliveryQueueMetricsSQL = `
SELECT DISTINCT
	ed.project_id,
	COALESCE(p.name, '') AS project_name,
	COALESCE(ed.endpoint_id, '') AS endpoint_id,
	ed.status,
	COALESCE(ed.event_type, '') AS event_type,
	COALESCE(e.source_id, 'http') AS source_id,
	COALESCE(p.organisation_id, '') AS organisation_id,
	COALESCE(o.name, '') AS organisation_name,
	COUNT(*) AS total
FROM convoy.event_deliveries ed
LEFT JOIN convoy.events e ON ed.event_id = e.id
LEFT JOIN convoy.projects p ON ed.project_id = p.id
LEFT JOIN convoy.organisations o ON p.organisation_id = o.id
WHERE ed.deleted_at IS NULL
GROUP BY ed.project_id, p.name, COALESCE(ed.endpoint_id, ''), ed.status, ed.event_type, e.source_id, p.organisation_id, o.name`

const eventEndpointBacklogMetricsSQL = `
WITH a1 AS (
	SELECT ed.project_id,
		   COALESCE(e.source_id, 'http') AS source_id,
		   COALESCE(ed.endpoint_id, '') AS endpoint_id,
		   EXTRACT(EPOCH FROM (NOW() - MIN(ed.created_at))) AS age_seconds
	FROM convoy.event_deliveries ed
	LEFT JOIN convoy.events e ON e.id = ed.event_id
	WHERE ed.status = 'Processing'
	GROUP BY ed.project_id, e.source_id, COALESCE(ed.endpoint_id, '')
	ORDER BY age_seconds DESC, ed.project_id, e.source_id, COALESCE(ed.endpoint_id, '')
	LIMIT 1000
)
SELECT project_id, source_id, endpoint_id, age_seconds
FROM (
	SELECT * FROM a1
	UNION ALL
	SELECT ed.project_id,
		   COALESCE(e.source_id, 'http'),
		   COALESCE(ed.endpoint_id, ''),
		   0 AS age_seconds
	FROM convoy.event_deliveries ed
	LEFT JOIN convoy.events e ON e.id = ed.event_id
	WHERE ed.status = 'Success'
	  AND NOT EXISTS (
		  SELECT 1 FROM a1
		  WHERE a1.project_id = ed.project_id
			AND a1.source_id = COALESCE(e.source_id, 'http')
			AND a1.endpoint_id = COALESCE(ed.endpoint_id, '')
	  )
	GROUP BY ed.project_id, e.source_id, COALESCE(ed.endpoint_id, '')
) AS combined
ORDER BY project_id, source_id, endpoint_id
LIMIT 1000`

// WriteQueueMetricsSnapshot rebuilds Prometheus snapshot tables. Each series
// is written independently so one timeout leaves the other gauges on the
// previous generation. Failure policy: fail open for Collect; a failed write
// keeps lastRun stamped by the caller so the next tick can retry.
func (p *Postgres) WriteQueueMetricsSnapshot(ctx context.Context) error {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, writeQueueMetricsTimeout)
		defer cancel()
	}

	var first error
	for _, step := range []func(context.Context) error{
		p.writeEventQueueSnapshot,
		p.writeEventQueueBacklogSnapshot,
		p.writeEventDeliveryQueueSnapshot,
		p.writeEventEndpointBacklogSnapshot,
	} {
		if err := step(ctx); err != nil {
			if first == nil {
				first = err
			}
			p.metricsLogError(fmt.Sprintf("queue metrics snapshot step failed: %v", err))
		}
	}
	return first
}

func (p *Postgres) writeEventQueueSnapshot(ctx context.Context) error {
	return p.replaceSnapshot(ctx, metricsSnapshotEventQueue, `
		INSERT INTO convoy.metrics_event_queue (generation, project_id, source_id, total)
		SELECT $1, project_id, source_id, total FROM (`+eventQueueMetricsSQL+`) src`,
		`DELETE FROM convoy.metrics_event_queue WHERE generation <> $1`)
}

func (p *Postgres) writeEventQueueBacklogSnapshot(ctx context.Context) error {
	return p.replaceSnapshot(ctx, metricsSnapshotEventQueueBacklog, `
		INSERT INTO convoy.metrics_event_queue_backlog (generation, project_id, source_id, age_seconds)
		SELECT $1, project_id, source_id, age_seconds FROM (`+eventQueueBacklogMetricsSQL+`) src`,
		`DELETE FROM convoy.metrics_event_queue_backlog WHERE generation <> $1`)
}

func (p *Postgres) writeEventDeliveryQueueSnapshot(ctx context.Context) error {
	return p.replaceSnapshot(ctx, metricsSnapshotEventDeliveryQueue, `
		INSERT INTO convoy.metrics_event_delivery_queue (
			generation, project_id, project_name, endpoint_id, status, event_type,
			source_id, organisation_id, organisation_name, total
		)
		SELECT $1, project_id, project_name, endpoint_id, status, event_type,
			source_id, organisation_id, organisation_name, total
		FROM (`+eventDeliveryQueueMetricsSQL+`) src`,
		`DELETE FROM convoy.metrics_event_delivery_queue WHERE generation <> $1`)
}

func (p *Postgres) writeEventEndpointBacklogSnapshot(ctx context.Context) error {
	return p.replaceSnapshot(ctx, metricsSnapshotEventEndpointBacklog, `
		INSERT INTO convoy.metrics_event_endpoint_backlog (generation, project_id, source_id, endpoint_id, age_seconds)
		SELECT $1, project_id, source_id, endpoint_id, age_seconds FROM (`+eventEndpointBacklogMetricsSQL+`) src`,
		`DELETE FROM convoy.metrics_event_endpoint_backlog WHERE generation <> $1`)
}

func (p *Postgres) replaceSnapshot(ctx context.Context, name, insertSQL, deleteSQL string) error {
	tx, err := p.GetDB().BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin %s snapshot: %w", name, err)
	}
	defer func() { _ = tx.Rollback() }()

	var current int64
	if err = tx.QueryRowContext(ctx, `
		SELECT generation FROM convoy.metrics_snapshot_meta WHERE name = $1`, name).Scan(&current); err != nil {
		return fmt.Errorf("load %s snapshot generation: %w", name, err)
	}
	next := current + 1

	if _, err = tx.ExecContext(ctx, insertSQL, next); err != nil {
		return fmt.Errorf("insert %s snapshot: %w", name, err)
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE convoy.metrics_snapshot_meta
		SET generation = $1, refreshed_at = NOW()
		WHERE name = $2`, next, name); err != nil {
		return fmt.Errorf("swing %s snapshot pointer: %w", name, err)
	}
	if _, err = tx.ExecContext(ctx, deleteSQL, next); err != nil {
		return fmt.Errorf("drop old %s snapshot: %w", name, err)
	}
	return tx.Commit()
}
