-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';

-- Queue the event-deliveries list/chart index instead of CREATE INDEX.
-- CONCURRENTLY is illegal on a partitioned parent, and a blocking build on
-- a large heap takes a write lock for the full scan. After migrate, convoy
-- server and convoy agent start this rebuild at boot (ahead of the payload
-- GIN) so the 30-day Event Deliveries page can range-scan (project_id,
-- created_at) and stop at LIMIT. Until it is valid the list still times out
-- fail-closed; ingest is unaffected.
INSERT INTO convoy.dropped_indexes (index_name, table_name, definition)
VALUES (
    'idx_event_deliveries_project_created_id_deleted',
    'event_deliveries',
    'CREATE INDEX idx_event_deliveries_project_created_id_deleted ON convoy.event_deliveries USING btree (project_id, created_at DESC, id DESC) WHERE (deleted_at IS NULL)'
)
ON CONFLICT (index_name) DO NOTHING;

RESET lock_timeout;
RESET statement_timeout;

-- +migrate Down
SET lock_timeout = '2s';
SET statement_timeout = '30s';

DELETE FROM convoy.dropped_indexes WHERE index_name = 'idx_event_deliveries_project_created_id_deleted';
DROP INDEX IF EXISTS convoy.idx_event_deliveries_project_created_id_deleted;

RESET lock_timeout;
RESET statement_timeout;
