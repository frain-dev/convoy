-- +migrate Up
ALTER TABLE convoy.configurations ADD COLUMN IF NOT EXISTS retention_policy_enabled BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE convoy.configurations ADD COLUMN IF NOT EXISTS retention_policy_policy TEXT NOT NULL DEFAULT false;


ALTER TABLE convoy.project_configurations DROP COLUMN IF EXISTS retention_policy_enabled;
ALTER TABLE convoy.project_configurations DROP COLUMN IF EXISTS retention_policy_policy;

-- +migrate Down
ALTER TABLE convoy.configurations DROP COLUMN IF EXISTS retention_policy_enabled;
ALTER TABLE convoy.configurations DROP COLUMN IF EXISTS retention_policy_policy;

ALTER TABLE convoy.project_configurations ADD COLUMN IF NOT EXISTS retention_policy_enabled BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE convoy.project_configurations ADD COLUMN IF NOT EXISTS retention_policy_policy TEXT NOT NULL DEFAULT false;
