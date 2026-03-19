-- +migrate Up
ALTER TABLE convoy.endpoints ADD COLUMN IF NOT EXISTS basic_auth_config JSONB;
ALTER TABLE convoy.endpoints ADD COLUMN IF NOT EXISTS basic_auth_config_cipher TEXT;

-- +migrate Down
ALTER TABLE convoy.endpoints DROP COLUMN IF EXISTS basic_auth_config;
ALTER TABLE convoy.endpoints DROP COLUMN IF EXISTS basic_auth_config_cipher;
