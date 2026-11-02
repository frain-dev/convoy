-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';

-- Queue list indexes for Event Deliveries status filters and Event Logs.
-- CREATE INDEX CONCURRENTLY is illegal on a partitioned parent and a blocking
-- build takes a write lock for the full scan. After migrate, convoy server and
-- convoy agent rebuild from dropped_indexes at boot.
--
-- Status-filtered paging probes each status with ORDER BY (created_at, id).
-- idx_event_deliveries_usage covers (project_id, status, created_at) but omits
-- id and may be missing after an operator drop. The list index matches the
-- keyset order. The endpoint variant covers endpoint+status combinations.
INSERT INTO convoy.dropped_indexes (index_name, table_name, definition)
VALUES
    (
        'idx_event_deliveries_list_status_created_id',
        'event_deliveries',
        'CREATE INDEX idx_event_deliveries_list_status_created_id ON convoy.event_deliveries USING btree (project_id, status, created_at DESC, id DESC) WHERE (deleted_at IS NULL)'
    ),
    (
        'idx_event_deliveries_list_endpoint_status_created_id',
        'event_deliveries',
        'CREATE INDEX idx_event_deliveries_list_endpoint_status_created_id ON convoy.event_deliveries USING btree (project_id, endpoint_id, status, created_at DESC, id DESC) WHERE (deleted_at IS NULL)'
    ),
    (
        'idx_events_list_project_id',
        'events',
        'CREATE INDEX idx_events_list_project_id ON convoy.events USING btree (project_id, id DESC) WHERE (deleted_at IS NULL)'
    ),
    (
        'idx_event_deliveries_usage',
        'event_deliveries',
        'CREATE INDEX idx_event_deliveries_usage ON convoy.event_deliveries USING btree (project_id, status, created_at) INCLUDE (event_bytes) WHERE (deleted_at IS NULL)'
    )
ON CONFLICT (index_name) DO NOTHING;

RESET lock_timeout;
RESET statement_timeout;

-- +migrate Down
SET lock_timeout = '2s';
SET statement_timeout = '30s';

DELETE FROM convoy.dropped_indexes WHERE index_name IN (
    'idx_event_deliveries_list_status_created_id',
    'idx_event_deliveries_list_endpoint_status_created_id',
    'idx_events_list_project_id',
    'idx_event_deliveries_usage'
);
DROP INDEX IF EXISTS convoy.idx_event_deliveries_list_status_created_id;
DROP INDEX IF EXISTS convoy.idx_event_deliveries_list_endpoint_status_created_id;
DROP INDEX IF EXISTS convoy.idx_events_list_project_id;
-- Do not drop idx_event_deliveries_usage on down: it may predate this migration.

RESET lock_timeout;
RESET statement_timeout;
