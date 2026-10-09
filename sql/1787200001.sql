-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';

-- Queue the payload GIN instead of CREATE INDEX. A blocking build on events
-- takes a write lock for the full scan, and CREATE INDEX CONCURRENTLY is
-- illegal on a partitioned parent. After migrate, convoy server and convoy
-- agent start the existing concurrent rebuild for this name only; other
-- dropped_indexes rows stay on the admin / utils indexes --rebuild path.
INSERT INTO convoy.dropped_indexes (index_name, table_name, definition)
VALUES (
    'idx_events_payload_gin',
    'events',
    'CREATE INDEX idx_events_payload_gin ON convoy.events USING gin (convoy.event_payload_jsonb(data) jsonb_path_ops) WHERE (deleted_at IS NULL)'
)
ON CONFLICT (index_name) DO NOTHING;

RESET lock_timeout;
RESET statement_timeout;

-- +migrate Down
SET lock_timeout = '2s';
SET statement_timeout = '30s';

DELETE FROM convoy.dropped_indexes WHERE index_name = 'idx_events_payload_gin';
DROP INDEX IF EXISTS convoy.idx_events_payload_gin;

RESET lock_timeout;
RESET statement_timeout;
