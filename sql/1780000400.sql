-- +migrate Up
-- Forward-only usage byte columns. Nullable on purpose: existing rows stay NULL
-- (not backfilled) and are excluded from forward usage reads. New writes populate
-- them so usage can be summed from columns instead of scanning payloads.
ALTER TABLE convoy.events ADD COLUMN IF NOT EXISTS raw_bytes BIGINT;
ALTER TABLE convoy.events ADD COLUMN IF NOT EXISTS data_bytes BIGINT;
ALTER TABLE convoy.event_deliveries ADD COLUMN IF NOT EXISTS event_bytes BIGINT;

-- +migrate Up notransaction
-- Covering indexes for the usage aggregation. The byte columns are INCLUDEd so the
-- recompute can read them from the index; as rows in the window get populated, the
-- COALESCE fallback stops touching payloads and reads converge to index-only.
-- CONCURRENTLY avoids locking writes on large tables.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_events_usage
    ON convoy.events (project_id, created_at)
    INCLUDE (raw_bytes, data_bytes)
    WHERE deleted_at IS NULL;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_event_deliveries_usage
    ON convoy.event_deliveries (project_id, status, created_at)
    INCLUDE (event_bytes)
    WHERE deleted_at IS NULL;

-- +migrate Down notransaction
DROP INDEX CONCURRENTLY IF EXISTS convoy.idx_event_deliveries_usage;
DROP INDEX CONCURRENTLY IF EXISTS convoy.idx_events_usage;

-- +migrate Down
-- squawk-ignore ban-drop-column
ALTER TABLE convoy.event_deliveries DROP COLUMN IF EXISTS event_bytes;
-- squawk-ignore ban-drop-column
ALTER TABLE convoy.events DROP COLUMN IF EXISTS data_bytes;
-- squawk-ignore ban-drop-column
ALTER TABLE convoy.events DROP COLUMN IF EXISTS raw_bytes;
