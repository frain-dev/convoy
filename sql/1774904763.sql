-- +migrate Up
ALTER TABLE convoy.configurations ADD COLUMN IF NOT EXISTS azure_account_name VARCHAR;
ALTER TABLE convoy.configurations ADD COLUMN IF NOT EXISTS azure_account_key VARCHAR;
ALTER TABLE convoy.configurations ADD COLUMN IF NOT EXISTS azure_container_name VARCHAR;
ALTER TABLE convoy.configurations ADD COLUMN IF NOT EXISTS azure_endpoint VARCHAR;
ALTER TABLE convoy.configurations ADD COLUMN IF NOT EXISTS azure_prefix VARCHAR;

-- +migrate Down
ALTER TABLE convoy.configurations DROP COLUMN IF EXISTS azure_account_name;
ALTER TABLE convoy.configurations DROP COLUMN IF EXISTS azure_account_key;
ALTER TABLE convoy.configurations DROP COLUMN IF EXISTS azure_container_name;
ALTER TABLE convoy.configurations DROP COLUMN IF EXISTS azure_endpoint;
ALTER TABLE convoy.configurations DROP COLUMN IF EXISTS azure_prefix;
