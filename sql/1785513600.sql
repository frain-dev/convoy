-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';

ALTER TABLE convoy.project_configurations
ADD COLUMN IF NOT EXISTS sync_dynamic_event_ack BOOLEAN NOT NULL DEFAULT FALSE;

RESET lock_timeout;
RESET statement_timeout;

-- +migrate Down
SET lock_timeout = '2s';
SET statement_timeout = '30s';

-- squawk-ignore ban-drop-column
ALTER TABLE convoy.project_configurations DROP COLUMN IF EXISTS sync_dynamic_event_ack;

RESET lock_timeout;
RESET statement_timeout;
