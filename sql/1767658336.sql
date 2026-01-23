-- +migrate Up

-- Create indexes on event_deliveries to optimize materialized view queries
CREATE INDEX IF NOT EXISTS idx_event_deliveries_status_processing_project_source_created 
ON convoy.event_deliveries (status, project_id, event_id, created_at) 
WHERE status = 'Processing';

CREATE INDEX IF NOT EXISTS idx_event_deliveries_status_processing_project_source_endpoint_created 
ON convoy.event_deliveries (status, project_id, endpoint_id, event_id, created_at) 
WHERE status = 'Processing';

CREATE INDEX IF NOT EXISTS idx_event_deliveries_status_success_project_source 
ON convoy.event_deliveries (status, project_id, event_id) 
WHERE status = 'Success';

-- Index for Success status with endpoint_id
CREATE INDEX IF NOT EXISTS idx_event_deliveries_status_success_project_source_endpoint 
ON convoy.event_deliveries (status, project_id, endpoint_id, event_id) 
WHERE status = 'Success';

-- Materialized view for event queue metrics
CREATE MATERIALIZED VIEW IF NOT EXISTS convoy.event_queue_metrics_mv AS
SELECT DISTINCT 
    project_id, 
    COALESCE(source_id, 'http') as source_id, 
    COUNT(*) as total 
FROM convoy.events 
GROUP BY project_id, source_id;

-- Create unique index for concurrent refresh and fast lookups
CREATE UNIQUE INDEX IF NOT EXISTS idx_event_queue_metrics_mv_unique 
ON convoy.event_queue_metrics_mv(project_id, source_id);

-- Materialized view for event delivery queue metrics
CREATE MATERIALIZED VIEW IF NOT EXISTS convoy.event_delivery_queue_metrics_mv AS
SELECT DISTINCT 
    ed.project_id, 
    COALESCE(p.name, '') as project_name,
    ed.endpoint_id, 
    ed.status,
    COALESCE(ed.event_type, '') as event_type,
    COALESCE(e.source_id, 'http') as source_id,
    COALESCE(p.organisation_id, '') as organisation_id,
    COALESCE(o.name, '') as organisation_name,
    COUNT(*) as total 
FROM convoy.event_deliveries ed
LEFT JOIN convoy.events e ON ed.event_id = e.id
LEFT JOIN convoy.projects p ON ed.project_id = p.id
LEFT JOIN convoy.organisations o ON p.organisation_id = o.id
WHERE ed.deleted_at IS NULL
GROUP BY ed.project_id, p.name, ed.endpoint_id, ed.status, ed.event_type, e.source_id, p.organisation_id, o.name;

-- Create unique index for concurrent refresh
CREATE UNIQUE INDEX IF NOT EXISTS idx_event_delivery_queue_metrics_mv_unique 
ON convoy.event_delivery_queue_metrics_mv(project_id, endpoint_id, status, event_type, source_id, organisation_id);

-- Materialized view for event queue backlog metrics
CREATE MATERIALIZED VIEW IF NOT EXISTS convoy.event_queue_backlog_metrics_mv AS
WITH a1 AS (
    SELECT ed.project_id,
           COALESCE(e.source_id, 'http') AS source_id,
           EXTRACT(EPOCH FROM (NOW() - MIN(ed.created_at))) AS age_seconds
    FROM convoy.event_deliveries ed
             LEFT JOIN convoy.events e ON e.id = ed.event_id
    WHERE ed.status = 'Processing'
    GROUP BY ed.project_id, e.source_id
    ORDER BY age_seconds DESC, ed.project_id, e.source_id
    LIMIT 1000 -- samples
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
LIMIT 1000; -- samples

-- Create unique index for concurrent refresh
CREATE UNIQUE INDEX IF NOT EXISTS idx_event_queue_backlog_metrics_mv_unique 
ON convoy.event_queue_backlog_metrics_mv(project_id, source_id);

-- Materialized view for endpoint backlog metrics
CREATE MATERIALIZED VIEW IF NOT EXISTS convoy.event_endpoint_backlog_metrics_mv AS
WITH a1 AS (
    SELECT ed.project_id,
           COALESCE(e.source_id, 'http') AS source_id,
           ed.endpoint_id,
           EXTRACT(EPOCH FROM (NOW() - MIN(ed.created_at))) AS age_seconds
    FROM convoy.event_deliveries ed
    LEFT JOIN convoy.events e ON e.id = ed.event_id
    WHERE ed.status = 'Processing'
    GROUP BY ed.project_id, e.source_id, ed.endpoint_id
    ORDER BY age_seconds DESC, ed.project_id, e.source_id, ed.endpoint_id
    LIMIT 1000 -- samples
)
SELECT project_id, source_id, endpoint_id, age_seconds
FROM (
    SELECT * FROM a1
    UNION ALL
    SELECT ed.project_id,
           COALESCE(e.source_id, 'http'),
           ed.endpoint_id,
           0 AS age_seconds
    FROM convoy.event_deliveries ed
    LEFT JOIN convoy.events e ON e.id = ed.event_id
    WHERE ed.status = 'Success' 
      AND NOT EXISTS (
          SELECT 1 FROM a1 
          WHERE a1.project_id = ed.project_id 
            AND a1.source_id = COALESCE(e.source_id, 'http')
            AND a1.endpoint_id = ed.endpoint_id
      )
    GROUP BY ed.project_id, e.source_id, ed.endpoint_id
) AS combined
ORDER BY project_id, source_id, endpoint_id
LIMIT 1000; -- samples

-- Create unique index for concurrent refresh
CREATE UNIQUE INDEX IF NOT EXISTS idx_event_endpoint_backlog_metrics_mv_unique 
ON convoy.event_endpoint_backlog_metrics_mv(project_id, source_id, endpoint_id);


REFRESH MATERIALIZED VIEW convoy.event_queue_metrics_mv;
REFRESH MATERIALIZED VIEW convoy.event_delivery_queue_metrics_mv;
REFRESH MATERIALIZED VIEW convoy.event_queue_backlog_metrics_mv;
REFRESH MATERIALIZED VIEW convoy.event_endpoint_backlog_metrics_mv;

-- +migrate Down

DROP INDEX IF EXISTS convoy.idx_event_endpoint_backlog_metrics_mv_unique;
DROP INDEX IF EXISTS convoy.idx_event_queue_backlog_metrics_mv_unique;
DROP INDEX IF EXISTS convoy.idx_event_delivery_queue_metrics_mv_unique;
DROP INDEX IF EXISTS convoy.idx_event_queue_metrics_mv_unique;

DROP MATERIALIZED VIEW IF EXISTS convoy.event_endpoint_backlog_metrics_mv;
DROP MATERIALIZED VIEW IF EXISTS convoy.event_queue_backlog_metrics_mv;
DROP MATERIALIZED VIEW IF EXISTS convoy.event_delivery_queue_metrics_mv;
DROP MATERIALIZED VIEW IF EXISTS convoy.event_queue_metrics_mv;

-- Drop indexes created for materialized view optimization
DROP INDEX IF EXISTS convoy.idx_event_deliveries_status_processing_project_source_created;
DROP INDEX IF EXISTS convoy.idx_event_deliveries_status_processing_project_source_endpoint_created;
DROP INDEX IF EXISTS convoy.idx_event_deliveries_status_success_project_source;
DROP INDEX IF EXISTS convoy.idx_event_deliveries_status_success_project_source_endpoint;
