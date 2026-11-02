-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';

ALTER TABLE convoy.project_configurations
ADD COLUMN IF NOT EXISTS add_event_id_trace_headers BOOLEAN NOT NULL DEFAULT FALSE;

RESET lock_timeout;
RESET statement_timeout;

-- +migrate Down
SET lock_timeout = '2s';
SET statement_timeout = '30s';

-- squawk-ignore ban-drop-column
ALTER TABLE convoy.project_configurations DROP COLUMN IF EXISTS add_event_id_trace_headers;

RESET lock_timeout;
RESET statement_timeout;
