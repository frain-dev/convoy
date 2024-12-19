-- +migrate Up
ALTER TABLE convoy.endpoints
    ADD COLUMN IF NOT EXISTS is_encrypted BOOLEAN DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS secrets_cipher bytea,
    ADD COLUMN IF NOT EXISTS authentication_type_api_key_header_value_cipher bytea;

CREATE INDEX idx_endpoints_is_encrypted ON convoy.endpoints (is_encrypted);

-- +migrate Down
DROP INDEX IF EXISTS idx_endpoints_is_encrypted;

ALTER TABLE convoy.endpoints
    DROP COLUMN IF EXISTS is_encrypted,
    DROP COLUMN IF EXISTS secrets_cipher,
    DROP COLUMN IF EXISTS authentication_type_api_key_header_value_cipher;
