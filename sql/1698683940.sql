-- +migrate Up
ALTER TABLE convoy.event_deliveries ADD COLUMN latency text;

-- +migrate Down
ALTER TABLE convoy.event_deliveries DROP COLUMN latency;

