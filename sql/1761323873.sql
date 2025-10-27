-- +migrate Up
ALTER TABLE convoy.endpoints
    ADD COLUMN IF NOT EXISTS mtls_client_cert JSONB;

-- +migrate Down
ALTER TABLE convoy.endpoints
    DROP COLUMN IF EXISTS mtls_client_cert;

