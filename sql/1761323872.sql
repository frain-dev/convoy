-- +migrate Up
ALTER TABLE convoy.endpoints
    ADD COLUMN IF NOT EXISTS mtls_client_cert_cipher bytea;

-- +migrate Down
ALTER TABLE convoy.endpoints
    DROP COLUMN IF EXISTS mtls_client_cert_cipher;
