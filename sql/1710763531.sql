-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';
ALTER TABLE convoy.project_configurations ADD COLUMN IF NOT EXISTS ssl_enforce_secure_endpoints BOOLEAN DEFAULT TRUE;

-- +migrate Down
-- squawk-ignore ban-drop-column
ALTER TABLE convoy.project_configurations DROP COLUMN IF EXISTS ssl_enforce_secure_endpoints;
