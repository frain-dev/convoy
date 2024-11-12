-- +migrate Up
CREATE INDEX IF NOT EXISTS idx_delivery_attempts_event_delivery_id_created_at_desc ON convoy.delivery_attempts(event_delivery_id, created_at DESC);

-- +migrate Down
DROP INDEX IF EXISTS convoy.idx_delivery_attempts_event_delivery_id_created_at_desc;
