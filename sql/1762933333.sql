-- +migrate Up
ALTER TABLE convoy.endpoints
    ADD COLUMN IF NOT EXISTS oauth2_config jsonb,
    ADD COLUMN IF NOT EXISTS oauth2_config_cipher bytea;

CREATE INDEX IF NOT EXISTS idx_endpoints_oauth2_config ON convoy.endpoints USING gin (oauth2_config) WHERE oauth2_config IS NOT NULL;

-- +migrate Down
DROP INDEX IF EXISTS idx_endpoints_oauth2_config;

ALTER TABLE convoy.endpoints
    DROP COLUMN IF EXISTS oauth2_config,
    DROP COLUMN IF EXISTS oauth2_config_cipher;

