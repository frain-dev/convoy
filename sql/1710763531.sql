-- +migrate Up
ALTER TABLE convoy.project_configurations ADD COLUMN IF NOT EXISTS ssl_enforce_secure_endpoints BOOLEAN;


-- +migrate Up
UPDATE convoy.project_configurations SET ssl_enforce_secure_endpoints = true;

-- +migrate Down
ALTER TABLE convoy.project_configurations DROP COLUMN IF EXISTS ssl_enforce_secure_endpoints;
