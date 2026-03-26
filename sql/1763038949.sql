-- +migrate Up notransaction
SET lock_timeout = '2s';
SET statement_timeout = '30s';
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_events_headers_broker_message_id ON
    convoy.events ((headers -> 'x-broker-message-id' ->> 0));

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_event_deliveries_headers_broker_message_id ON
    convoy.event_deliveries ((headers -> 'x-broker-message-id' ->> 0));

-- +migrate Down
DROP INDEX CONCURRENTLY IF EXISTS convoy.idx_events_headers_broker_message_id;
DROP INDEX CONCURRENTLY IF EXISTS convoy.idx_event_deliveries_headers_broker_message_id;
