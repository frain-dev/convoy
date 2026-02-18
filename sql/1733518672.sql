-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';
ALTER TABLE convoy.endpoints
    ADD COLUMN IF NOT EXISTS is_encrypted BOOLEAN DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS secrets_cipher bytea,
    ADD COLUMN IF NOT EXISTS authentication_type_api_key_header_value_cipher bytea;

-- +migrate Up notransaction
SET lock_timeout = '2s';
SET statement_timeout = '30s';
CREATE INDEX CONCURRENTLY idx_endpoints_is_encrypted ON convoy.endpoints (is_encrypted);

-- +migrate Down notransaction
DROP INDEX CONCURRENTLY IF EXISTS idx_endpoints_is_encrypted;

-- +migrate Down
ALTER TABLE convoy.endpoints
    DROP COLUMN IF EXISTS is_encrypted,
    DROP COLUMN IF EXISTS secrets_cipher,
    DROP COLUMN IF EXISTS authentication_type_api_key_header_value_cipher;
