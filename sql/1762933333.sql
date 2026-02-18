-- +migrate Up
SET lock_timeout = '2s';
SET statement_timeout = '30s';
ALTER TABLE convoy.endpoints
    ADD COLUMN IF NOT EXISTS oauth2_config jsonb,
    ADD COLUMN IF NOT EXISTS oauth2_config_cipher bytea;

-- +migrate Up notransaction
SET lock_timeout = '2s';
SET statement_timeout = '30s';
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_endpoints_oauth2_config ON convoy.endpoints USING gin (oauth2_config) WHERE oauth2_config IS NOT NULL;

-- +migrate Down notransaction
DROP INDEX CONCURRENTLY IF EXISTS idx_endpoints_oauth2_config;

-- +migrate Down
ALTER TABLE convoy.endpoints
    DROP COLUMN IF EXISTS oauth2_config,
    DROP COLUMN IF EXISTS oauth2_config_cipher;

