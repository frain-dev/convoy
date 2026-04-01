-- +migrate Up
CREATE PUBLICATION convoy_backup FOR TABLE
    convoy.events,
    convoy.event_deliveries,
    convoy.delivery_attempts;

-- +migrate Down
DROP PUBLICATION IF EXISTS convoy_backup;
