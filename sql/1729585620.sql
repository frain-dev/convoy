-- +migrate Up notransaction
SET lock_timeout = '2s';
SET statement_timeout = '30s';
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_delivery_attempts_event_delivery_id_created_at_desc ON convoy.delivery_attempts(event_delivery_id, created_at DESC);

-- +migrate Down
DROP INDEX CONCURRENTLY IF EXISTS convoy.idx_delivery_attempts_event_delivery_id_created_at_desc;
