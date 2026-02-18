-- squawk-ignore-file ban-drop-column
-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';
ALTER TABLE convoy.endpoints
    ADD COLUMN IF NOT EXISTS mtls_client_cert JSONB;

-- +migrate Down
-- squawk-ignore ban-drop-column
ALTER TABLE convoy.endpoints
    DROP COLUMN IF EXISTS mtls_client_cert;

