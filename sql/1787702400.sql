-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';

-- Queue the Event Deliveries Observed-types index instead of CREATE INDEX.
-- DISTINCT over the date window scans every live delivery in range and 504s
-- the 5s list budget on large projects. A (project_id, event_type, created_at)
-- btree lets Observed walk one type at a time. CONCURRENTLY is illegal on a
-- partitioned parent, and a blocking build takes a write lock for the full
-- scan. After migrate, convoy server and convoy agent start this rebuild at
-- boot, after the list/chart index.
INSERT INTO convoy.dropped_indexes (index_name, table_name, definition)
VALUES (
    'idx_event_deliveries_project_event_type_created',
    'event_deliveries',
    'CREATE INDEX idx_event_deliveries_project_event_type_created ON convoy.event_deliveries USING btree (project_id, event_type, created_at) WHERE (deleted_at IS NULL)'
)
ON CONFLICT (index_name) DO NOTHING;

RESET lock_timeout;
RESET statement_timeout;

-- +migrate Down
SET lock_timeout = '2s';
SET statement_timeout = '30s';

DELETE FROM convoy.dropped_indexes WHERE index_name = 'idx_event_deliveries_project_event_type_created';
DROP INDEX IF EXISTS convoy.idx_event_deliveries_project_event_type_created;

RESET lock_timeout;
RESET statement_timeout;
