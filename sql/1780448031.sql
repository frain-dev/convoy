-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';

ALTER TABLE convoy.project_configurations
ADD COLUMN IF NOT EXISTS request_id_header TEXT NOT NULL DEFAULT 'X-Convoy-Idempotency-Key';

RESET lock_timeout;
RESET statement_timeout;

-- +migrate Down
SET lock_timeout = '2s';
SET statement_timeout = '30s';

-- squawk-ignore ban-drop-column
ALTER TABLE convoy.project_configurations DROP COLUMN IF EXISTS request_id_header;

RESET lock_timeout;
RESET statement_timeout;
