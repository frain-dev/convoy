-- +migrate Up
CREATE TABLE convoy.events_endpoints_temp (LIKE convoy.events_endpoints INCLUDING INDEXES INCLUDING CONSTRAINTS);
ALTER TABLE convoy.events_endpoints RENAME TO events_endpoints_deprecated;
CREATE UNIQUE INDEX IF NOT EXISTS idx_uq_constraint_events_endpoints_event_id_endpoint_id
    ON convoy.events_endpoints_temp(event_id, endpoint_id) NULLS NOT DISTINCT;

INSERT INTO convoy.events_endpoints_temp
SELECT DISTINCT ON (event_id, endpoint_id) *
FROM convoy.events_endpoints_deprecated ON CONFLICT DO NOTHING;

-- safely DROP convoy.events_endpoints_deprecated; if need be
ALTER TABLE convoy.events_endpoints_temp RENAME TO events_endpoints;


ALTER TABLE convoy.events ADD COLUMN IF NOT EXISTS status text DEFAULT NULL;
ALTER TABLE convoy.events ADD COLUMN IF NOT EXISTS metadata text DEFAULT NULL;

-- +migrate Down
DROP INDEX IF EXISTS idx_uq_constraint_events_endpoints_event_id_endpoint_id;

DROP TABLE IF EXISTS convoy.events_endpoints_deprecated;

ALTER TABLE convoy.events DROP COLUMN IF EXISTS status;
ALTER TABLE convoy.events DROP COLUMN IF EXISTS metadata;