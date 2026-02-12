-- +migrate Up
-- --- Events: created_at NOT NULL, drop redundant index, add list/pagination/source indexes ---
ALTER TABLE convoy.events
    ALTER COLUMN created_at SET NOT NULL;

DROP INDEX IF EXISTS convoy.idx_events_source_id_key;

CREATE INDEX IF NOT EXISTS idx_events_project_created_desc_not_deleted
    ON convoy.events (project_id, created_at DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_events_project_created_pagination
    ON convoy.events (project_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_events_source_lookup
    ON convoy.events (id, source_id);

-- --- Event deliveries: stats targets, composite index, index rename, drop FK, analyze ---
ALTER TABLE convoy.event_deliveries
    ALTER COLUMN project_id SET STATISTICS 1000,
    ALTER COLUMN endpoint_id SET STATISTICS 1000,
    ALTER COLUMN created_at SET STATISTICS 1000;

CREATE INDEX IF NOT EXISTS idx_event_deliveries_not_deleted
    ON convoy.event_deliveries (project_id, endpoint_id, status, event_type, event_id)
    WHERE deleted_at IS NULL;

ALTER INDEX IF EXISTS convoy.event_deliveries_event_type_1
    RENAME TO event_deliveries_event_type;

ALTER TABLE convoy.event_deliveries
    DROP CONSTRAINT IF EXISTS event_deliveries_event_id_fkey;

ANALYZE convoy.event_deliveries (project_id, endpoint_id, created_at);

-- +migrate Down
-- --- Event deliveries: revert stats, drop index, rename back, re-add FK ---
ALTER TABLE convoy.event_deliveries
    ALTER COLUMN project_id SET STATISTICS -1,
    ALTER COLUMN endpoint_id SET STATISTICS -1,
    ALTER COLUMN created_at SET STATISTICS -1;

DROP INDEX IF EXISTS convoy.idx_event_deliveries_not_deleted;

ALTER INDEX IF EXISTS convoy.event_deliveries_event_type
    RENAME TO event_deliveries_event_type_1;

ALTER TABLE convoy.event_deliveries
    ADD CONSTRAINT event_deliveries_event_id_fkey
    FOREIGN KEY (event_id) REFERENCES convoy.events(id);

-- --- Events: drop indexes, created_at nullable ---
DROP INDEX IF EXISTS convoy.idx_events_source_lookup;
DROP INDEX IF EXISTS convoy.idx_events_project_created_pagination;
DROP INDEX IF EXISTS convoy.idx_events_project_created_desc_not_deleted;

ALTER TABLE convoy.events
    ALTER COLUMN created_at DROP NOT NULL;
